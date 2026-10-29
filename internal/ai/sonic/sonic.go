// Package sonic connects Sela's voice room to Amazon Nova 2.5 Sonic, a
// speech-to-speech model on Amazon Bedrock (SPEC C27, owner decision D-058).
//
// Sonic is reached only through InvokeModelWithBidirectionalStream, which
// requires IAM (SigV4) credentials: Bedrock API keys are refused for this
// operation, so the client forces SigV4 even when AWS_BEARER_TOKEN_BEDROCK is
// present in the environment.
//
// The package is transport only. It turns Sonic's event stream into a small
// set of typed events; consent, age band, the crisis router and the release
// gate for assistant audio live in the HTTP layer.
package sonic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

// Defaults.
const (
	DefaultModel  = "amazon.nova-2-5-sonic"
	DefaultRegion = "us-east-1"
	DefaultVoice  = "tiffany"
	// InputSampleRate is the microphone PCM rate Sela sends (mono int16).
	InputSampleRate = 16000
	// OutputSampleRate is the assistant PCM rate Sonic returns (mono int16).
	OutputSampleRate = 24000
	// TestDoubleModel names the labelled stand-in; it is never Sonic.
	TestDoubleModel = "test-double-voice"
)

// Role of a content block.
type Role string

// Roles Sonic reports.
const (
	RoleUser      Role = "USER"
	RoleAssistant Role = "ASSISTANT"
)

// Kind is the type of an Event.
type Kind int

// Event kinds.
const (
	// KindUserTranscript carries Sonic's transcription of the person's speech.
	KindUserTranscript Kind = iota + 1
	// KindAssistantText carries assistant text. Speculative text precedes
	// the matching audio; final text follows it.
	KindAssistantText
	// KindAssistantAudio carries assistant PCM (24 kHz mono int16).
	KindAssistantAudio
	// KindAssistantAudioEnd marks the end of one assistant audio block.
	KindAssistantAudioEnd
	// KindInterrupted means the person spoke over the assistant (barge-in).
	KindInterrupted
)

// Event is one decoded item from the model stream.
type Event struct {
	Kind        Kind
	ContentID   string
	Text        string
	Speculative bool
	PCM         []byte
	StopReason  string
}

// Session is one live voice conversation.
type Session interface {
	// SendAudio streams microphone PCM (16 kHz mono int16).
	SendAudio(pcm []byte) error
	// Events delivers decoded events; it is closed when the stream ends.
	Events() <-chan Event
	// Err reports why the stream ended, if it ended with an error.
	Err() error
	// Close ends the session politely and releases the stream.
	Close() error
}

// Opener starts sessions.
type Opener interface {
	Open(ctx context.Context, opt SessionOptions) (Session, error)
	// Model names the model behind the opener, for provenance.
	Model() string
}

// SessionOptions configure one session.
type SessionOptions struct {
	// System is the system prompt.
	System string
	// Voice overrides the configured voice.
	Voice string
}

// ErrClosed is returned when sending on a closed session.
var ErrClosed = errors.New("sonic: session closed")

// decoder turns raw Sonic output events into Events. It keeps the role and
// generation stage of each open content block, because textOutput and
// audioOutput events refer back to them by contentId.
type decoder struct {
	blocks map[string]block
}

type block struct {
	role        Role
	kind        string // TEXT or AUDIO
	speculative bool
}

func newDecoder() *decoder { return &decoder{blocks: map[string]block{}} }

type rawEvent struct {
	Event struct {
		ContentStart *struct {
			ContentID             string `json:"contentId"`
			Role                  Role   `json:"role"`
			Type                  string `json:"type"`
			AdditionalModelFields string `json:"additionalModelFields"`
		} `json:"contentStart"`
		TextOutput *struct {
			ContentID string `json:"contentId"`
			Role      Role   `json:"role"`
			Content   string `json:"content"`
		} `json:"textOutput"`
		AudioOutput *struct {
			ContentID string `json:"contentId"`
			Content   string `json:"content"`
		} `json:"audioOutput"`
		ContentEnd *struct {
			ContentID  string `json:"contentId"`
			Type       string `json:"type"`
			StopReason string `json:"stopReason"`
		} `json:"contentEnd"`
	} `json:"event"`
}

// decode returns the events carried by one raw payload (zero or more).
func (d *decoder) decode(payload []byte) ([]Event, error) {
	var r rawEvent
	if err := json.Unmarshal(payload, &r); err != nil {
		return nil, err
	}
	e := r.Event
	switch {
	case e.ContentStart != nil:
		cs := e.ContentStart
		b := block{role: cs.Role, kind: cs.Type}
		if cs.AdditionalModelFields != "" {
			var f struct {
				GenerationStage string `json:"generationStage"`
			}
			if json.Unmarshal([]byte(cs.AdditionalModelFields), &f) == nil {
				b.speculative = f.GenerationStage == "SPECULATIVE"
			}
		}
		d.blocks[cs.ContentID] = b
		return nil, nil
	case e.TextOutput != nil:
		t := e.TextOutput
		if isInterruption(t.Content) {
			return []Event{{Kind: KindInterrupted, ContentID: t.ContentID}}, nil
		}
		b := d.blocks[t.ContentID]
		role := t.Role
		if role == "" {
			role = b.role
		}
		if role == RoleUser {
			return []Event{{Kind: KindUserTranscript, ContentID: t.ContentID, Text: t.Content}}, nil
		}
		return []Event{{Kind: KindAssistantText, ContentID: t.ContentID, Text: t.Content, Speculative: b.speculative}}, nil
	case e.AudioOutput != nil:
		a := e.AudioOutput
		pcm, err := base64.StdEncoding.DecodeString(a.Content)
		if err != nil {
			return nil, err
		}
		return []Event{{Kind: KindAssistantAudio, ContentID: a.ContentID, PCM: pcm}}, nil
	case e.ContentEnd != nil:
		ce := e.ContentEnd
		b := d.blocks[ce.ContentID]
		delete(d.blocks, ce.ContentID)
		if ce.StopReason == "INTERRUPTED" {
			return []Event{{Kind: KindInterrupted, ContentID: ce.ContentID, StopReason: ce.StopReason}}, nil
		}
		if b.kind == "AUDIO" || ce.Type == "AUDIO" {
			return []Event{{Kind: KindAssistantAudioEnd, ContentID: ce.ContentID, StopReason: ce.StopReason}}, nil
		}
		return nil, nil
	}
	return nil, nil
}

// isInterruption detects Sonic's barge-in marker in a text block.
func isInterruption(s string) bool {
	return strings.Contains(strings.ReplaceAll(s, " ", ""), `{"interrupted":true}`)
}
