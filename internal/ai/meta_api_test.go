package ai

import (
	"strings"
	"testing"

	"github.com/YanIanZ/sela/internal/llm"
)

func TestMetaAPIGeneratorRequiresMetaHost(t *testing.T) {
	cfg := llm.Config{
		Mode: llm.ModeMeta, BaseURL: "https://api.meta.ai/v1", APIKey: "test-key",
		ServedModel: "muse-spark-1.3", ModelID: "muse-spark-1.3", Revision: "muse-spark-1.3",
	}
	g, err := buildMetaGenerator(cfg, nil, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if id := g.Identity(); id.Role != ProviderMeta || id.Label() != ProviderMetaAPI || id.TestDouble || id.Disabled || id.ModelID != "muse-spark-1.3" {
		t.Fatalf("identity = %+v label=%s", id, id.Label())
	}
	for _, bad := range []string{"https://example.com/v1", "http://api.meta.ai/v1", "https://api.llama.com/v1"} {
		cfg.BaseURL = bad
		if _, err := buildMetaGenerator(cfg, nil, "", false); err == nil || !strings.Contains(err.Error(), "api.meta.ai") {
			t.Errorf("%s: %v", bad, err)
		}
	}
}
