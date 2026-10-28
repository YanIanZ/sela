package main

import (
	"errors"
	"os"
	"strings"

	"github.com/YanIanZ/sela/internal/ai/sonic"
	"github.com/YanIanZ/sela/internal/config"
)

// voiceOpener reads the voice-room settings (SPEC C27):
//
//	SELA_VOICE_MODE    disabled (default) | bedrock | test_double
//	SELA_VOICE_REGION  AWS region (default us-east-1)
//	SELA_VOICE_MODEL   default amazon.nova-2-5-sonic
//	SELA_VOICE_ID      Sonic voice (default tiffany)
//	AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN (optional)
//
// bedrock needs IAM credentials: Bedrock API keys
// (AWS_BEARER_TOKEN_BEDROCK) cannot open a Sonic stream and are ignored.
// test_double needs SELA_ALLOW_TEST_DOUBLE=true outside production, staging
// and demo. Errors name variables, never values.
func voiceOpener(env string) (sonic.Opener, string, error) {
	get := func(k string) string { return strings.TrimSpace(os.Getenv(k)) }
	mode := get("SELA_VOICE_MODE")
	if mode == "" {
		mode = "disabled"
	}
	switch mode {
	case "disabled":
		return nil, mode, nil
	case "test_double":
		public := env == config.EnvProduction || env == config.EnvStaging || env == config.EnvDemo
		if get("SELA_ALLOW_TEST_DOUBLE") != "true" || public {
			return nil, mode, errors.New("SELA_VOICE_MODE=test_double requires SELA_ALLOW_TEST_DOUBLE=true and SELA_ENV development or test")
		}
		return sonic.TestDouble{}, mode, nil
	case "bedrock":
		o, err := sonic.NewBedrock(sonic.Config{
			Region: get("SELA_VOICE_REGION"), Model: get("SELA_VOICE_MODEL"), Voice: get("SELA_VOICE_ID"),
			AccessKeyID: get("AWS_ACCESS_KEY_ID"), SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
			SessionToken: os.Getenv("AWS_SESSION_TOKEN"),
		})
		if err != nil {
			return nil, mode, err
		}
		return o, mode, nil
	}
	return nil, mode, errors.New("SELA_VOICE_MODE: must be disabled, bedrock or test_double")
}
