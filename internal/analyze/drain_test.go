package analyze

import (
	"strings"
	"testing"
)

func TestMineTemplates(t *testing.T) {
	tests := []struct {
		name          string
		records       []LogRecord
		wantTemplates int
		wantTop       string // expected top (highest-bytes) template text
		wantTopExact  bool
	}{
		{
			name: "variable id positions become wildcards",
			records: []LogRecord{
				{Service: "catalog", Severity: "DEBUG", Body: "cache miss for product id=1 refreshing", Weight: 5},
				{Service: "catalog", Severity: "DEBUG", Body: "cache miss for product id=2 refreshing", Weight: 5},
				{Service: "catalog", Severity: "DEBUG", Body: "cache miss for product id=3 refreshing", Weight: 5},
			},
			wantTemplates: 1,
			wantTop:       "cache miss for product <*> refreshing",
			wantTopExact:  false,
		},
		{
			name: "identical lines stay exact",
			records: []LogRecord{
				{Service: "catalog", Severity: "DEBUG", Body: "pool stats: active=10 idle=5", Weight: 3},
				{Service: "catalog", Severity: "DEBUG", Body: "pool stats: active=10 idle=5", Weight: 4},
			},
			wantTemplates: 1,
			wantTop:       "pool stats: active=10 idle=5",
			wantTopExact:  true,
		},
		{
			name: "different token counts never merge",
			records: []LogRecord{
				{Service: "a", Body: "one two three", Weight: 1},
				{Service: "a", Body: "one two three four", Weight: 1},
			},
			wantTemplates: 2,
		},
		{
			name: "different services never merge",
			records: []LogRecord{
				{Service: "a", Body: "request completed ok", Weight: 1},
				{Service: "b", Body: "request completed ok", Weight: 1},
			},
			wantTemplates: 2,
		},
		{
			name: "dissimilar lines of equal length stay apart",
			records: []LogRecord{
				{Service: "a", Body: "user login succeeded for alice", Weight: 1},
				{Service: "a", Body: "disk pressure evicting pod kube-1", Weight: 1},
			},
			wantTemplates: 2,
		},
		{
			name:          "empty bodies are skipped",
			records:       []LogRecord{{Service: "a", Body: "   ", Weight: 1}},
			wantTemplates: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MineTemplates(tt.records, DefaultDrainConfig())
			if len(got) != tt.wantTemplates {
				t.Fatalf("got %d templates, want %d: %+v", len(got), tt.wantTemplates, got)
			}
			if tt.wantTop != "" {
				if got[0].Text != tt.wantTop {
					t.Errorf("top template = %q, want %q", got[0].Text, tt.wantTop)
				}
				if got[0].Exact != tt.wantTopExact {
					t.Errorf("top template Exact = %v, want %v", got[0].Exact, tt.wantTopExact)
				}
			}
		})
	}
}

func TestMineTemplatesWeightsAndRanking(t *testing.T) {
	records := []LogRecord{
		{Service: "svc", Severity: "DEBUG", Body: "big template line number 1", Weight: 100},
		{Service: "svc", Severity: "DEBUG", Body: "big template line number 2", Weight: 100},
		{Service: "svc", Severity: "INFO", Body: "tiny", Weight: 1},
	}
	got := MineTemplates(records, DefaultDrainConfig())
	if len(got) != 2 {
		t.Fatalf("got %d templates, want 2", len(got))
	}
	if got[0].Count != 200 {
		t.Errorf("weighted count = %d, want 200", got[0].Count)
	}
	// Ranking is by bytes: the weighted template must outrank the tiny one.
	if !strings.HasPrefix(got[0].Text, "big template") {
		t.Errorf("ranking wrong: top is %q", got[0].Text)
	}
	wantBytes := int64(len("big template line number 1")) * 200
	if got[0].Bytes != wantBytes {
		t.Errorf("weighted bytes = %d, want %d", got[0].Bytes, wantBytes)
	}
}

func TestMineTemplatesDeterministic(t *testing.T) {
	records := []LogRecord{
		{Service: "s", Body: "alpha beta gamma 1", Weight: 2},
		{Service: "s", Body: "alpha beta gamma 2", Weight: 3},
		{Service: "s", Body: "wholly different line here", Weight: 1},
	}
	a := MineTemplates(records, DefaultDrainConfig())
	b := MineTemplates(records, DefaultDrainConfig())
	if len(a) != len(b) {
		t.Fatalf("non-deterministic template count")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("non-deterministic output at %d: %+v vs %+v", i, a[i], b[i])
		}
	}
}
