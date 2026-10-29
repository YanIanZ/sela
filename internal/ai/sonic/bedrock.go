package sonic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// Config configures the Bedrock opener. The keys are IAM credentials; they
// are never logged.
type Config struct {
	Region       string
	Model        string
	Voice        string
	AccessKeyID  string
	SecretKey    string
	SessionToken string
}

// Validate reports missing required settings by name, never by value.
func (c Config) Validate() error {
	var missing []string
	if c.AccessKeyID == "" {
		missing = append(missing, "AWS_ACCESS_KEY_ID")
	}
	if c.SecretKey == "" {
		missing = append(missing, "AWS_SECRET_ACCESS_KEY")
	}
	if len(missing) > 0 {
		return fmt.Errorf("sonic: %s required (IAM credentials; Bedrock API keys cannot open a Sonic stream)", strings.Join(missing, " and "))
	}
	return nil
}

// Bedrock opens Sonic sessions on Amazon Bedrock.
type Bedrock struct {
	cfg    Config
	client *bedrockruntime.Client
}

// NewBedrock builds the opener. Credentials are static and explicit, and the
// auth scheme is pinned to SigV4 so a bearer token in the environment can
// never be chosen for the bidirectional stream (which refuses it).
func NewBedrock(cfg Config) (*Bedrock, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Region == "" {
		cfg.Region = DefaultRegion
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.Voice == "" {
		cfg.Voice = DefaultVoice
	}
	awsCfg := aws.Config{
		Region:      cfg.Region,
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretKey, cfg.SessionToken)),
	}
	client := bedrockruntime.NewFromConfig(awsCfg, func(o *bedrockruntime.Options) {
		o.AuthSchemePreference = []string{"sigv4"}
		o.BearerAuthTokenProvider = nil
	})
	return &Bedrock{cfg: cfg, client: client}, nil
}

// Model implements Opener.
func (b *Bedrock) Model() string { return b.cfg.Model }

// Open implements Opener.
func (b *Bedrock) Open(ctx context.Context, opt SessionOptions) (Session, error) {
	out, err := b.client.InvokeModelWithBidirectionalStream(ctx, &bedrockruntime.InvokeModelWithBidirectionalStreamInput{
		ModelId: aws.String(b.cfg.Model),
	})
	if err != nil {
		return nil, err
	}
	voice := opt.Voice
	if voice == "" {
		voice = b.cfg.Voice
	}
	s := &bedrockSession{
		stream:    out.GetStream(),
		ctx:       ctx,
		events:    make(chan Event, 256),
		prompt:    "sela-prompt",
		mic:       "sela-mic",
		lastAudio: time.Now(),
		stop:      make(chan struct{}),
	}
	if err := s.start(opt.System, voice); err != nil {
		_ = s.stream.Close()
		return nil, err
	}
	go s.read()
	go s.keepAlive()
	return s, nil
}

type bedrockSession struct {
	stream *bedrockruntime.InvokeModelWithBidirectionalStreamEventStream
	ctx    context.Context
	events chan Event
	prompt string
	mic    string

	mu        sync.Mutex
	closed    bool
	lastAudio time.Time
	err       error
	stop      chan struct{}
	stopOnce  sync.Once
}

func (s *bedrockSession) send(m map[string]any) error {
	payload, err := json.Marshal(map[string]any{"event": m})
	if err != nil {
		return err
	}
	return s.stream.Send(s.ctx, &types.InvokeModelWithBidirectionalStreamInputMemberChunk{
		Value: types.BidirectionalInputPayloadPart{Bytes: payload},
	})
}

func (s *bedrockSession) start(system, voice string) error {
	steps := []map[string]any{
		{"sessionStart": map[string]any{"inferenceConfiguration": map[string]any{"maxTokens": 1024, "topP": 0.9, "temperature": 0.6}}},
		{"promptStart": map[string]any{
			"promptName":              s.prompt,
			"textOutputConfiguration": map[string]any{"mediaType": "text/plain"},
			"audioOutputConfiguration": map[string]any{
				"mediaType": "audio/lpcm", "sampleRateHertz": OutputSampleRate, "sampleSizeBits": 16,
				"channelCount": 1, "voiceId": voice, "encoding": "base64", "audioType": "SPEECH",
			},
		}},
		{"contentStart": map[string]any{"promptName": s.prompt, "contentName": "sela-system", "type": "TEXT", "interactive": false, "role": "SYSTEM", "textInputConfiguration": map[string]any{"mediaType": "text/plain"}}},
		{"textInput": map[string]any{"promptName": s.prompt, "contentName": "sela-system", "content": system}},
		{"contentEnd": map[string]any{"promptName": s.prompt, "contentName": "sela-system"}},
		{"contentStart": map[string]any{"promptName": s.prompt, "contentName": s.mic, "type": "AUDIO", "interactive": true, "role": "USER",
			"audioInputConfiguration": map[string]any{"mediaType": "audio/lpcm", "sampleRateHertz": InputSampleRate, "sampleSizeBits": 16, "channelCount": 1, "audioType": "SPEECH", "encoding": "base64"}}},
	}
	for _, m := range steps {
		if err := s.send(m); err != nil {
			return err
		}
	}
	return nil
}

// SendAudio implements Session.
func (s *bedrockSession) SendAudio(pcm []byte) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrClosed
	}
	s.lastAudio = time.Now()
	s.mu.Unlock()
	return s.sendAudio(pcm)
}

func (s *bedrockSession) sendAudio(pcm []byte) error {
	return s.send(map[string]any{"audioInput": map[string]any{
		"promptName": s.prompt, "contentName": s.mic, "content": base64.StdEncoding.EncodeToString(pcm),
	}})
}

// keepAlive feeds silence while the browser sends nothing (muted mic, tab in
// the background): Sonic expects a continuous audio stream.
func (s *bedrockSession) keepAlive() {
	silence := make([]byte, InputSampleRate*2*32/1000) // 32 ms
	t := time.NewTicker(32 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.mu.Lock()
			idle := !s.closed && time.Since(s.lastAudio) > 120*time.Millisecond
			s.mu.Unlock()
			if idle {
				_ = s.sendAudio(silence)
			}
		}
	}
}

func (s *bedrockSession) read() {
	defer close(s.events)
	d := newDecoder()
	for raw := range s.stream.Events() {
		chunk, ok := raw.(*types.InvokeModelWithBidirectionalStreamOutputMemberChunk)
		if !ok {
			continue
		}
		evs, err := d.decode(chunk.Value.Bytes)
		if err != nil {
			continue
		}
		for _, e := range evs {
			select {
			case s.events <- e:
			case <-s.stop:
				return
			}
		}
	}
	if err := s.stream.Err(); err != nil {
		s.mu.Lock()
		s.err = err
		s.mu.Unlock()
	}
}

// Events implements Session.
func (s *bedrockSession) Events() <-chan Event { return s.events }

// Err implements Session.
func (s *bedrockSession) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Close implements Session.
func (s *bedrockSession) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()
	s.stopOnce.Do(func() { close(s.stop) })
	var errs []error
	for _, m := range []map[string]any{
		{"contentEnd": map[string]any{"promptName": s.prompt, "contentName": s.mic}},
		{"promptEnd": map[string]any{"promptName": s.prompt}},
		{"sessionEnd": map[string]any{}},
	} {
		if err := s.send(m); err != nil {
			errs = append(errs, err)
			break
		}
	}
	errs = append(errs, s.stream.Close())
	return errors.Join(errs...)
}
