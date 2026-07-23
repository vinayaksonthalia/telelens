package signozapi

import (
	"context"
	"testing"
)

func TestParseListResponse(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantLen   int
		wantTitle string
	}{
		{"bare array", `[{"title":"A"},{"title":"B"}]`, 2, "A"},
		{"data envelope", `{"status":"success","data":[{"title":"Dash"}]}`, 1, "Dash"},
		{"nested rules envelope", `{"data":{"rules":[{"alert":"Spike"}]}}`, 1, "Spike"},
		{"empty data", `{"data":[]}`, 0, ""},
		{"garbage yields empty, not error", `"just a string"`, 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			docs, err := parseListResponse([]byte(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			if len(docs) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(docs), tt.wantLen)
			}
			if tt.wantLen > 0 && docs[0].Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", docs[0].Title, tt.wantTitle)
			}
		})
	}
}

func TestFixtureAPI(t *testing.T) {
	api := &FixtureAPI{Dir: "../../fixtures"}
	dashboards, err := api.Dashboards(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(dashboards) == 0 {
		t.Error("no fixture dashboards")
	}
	rules, err := api.Rules(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) == 0 {
		t.Error("no fixture rules")
	}
}
