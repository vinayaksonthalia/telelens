// Package config resolves TELELENS settings from flags and environment
// variables (documented in .env.example). Flags win over env vars.
package config

import (
	"os"
	"strconv"

	"github.com/vinayaksonthalia/telelens/internal/analyze"
)

// Config is the resolved runtime configuration.
type Config struct {
	// FixtureDir, when non-empty, selects offline fixture mode.
	FixtureDir string
	// ClickHouseURL is the ClickHouse HTTP interface (live mode).
	ClickHouseURL      string
	ClickHouseUser     string
	ClickHousePassword string
	// SignozURL is the SigNoz API base URL (live mode, GET only).
	SignozURL    string
	SignozAPIKey string
	// WindowDays is the scan window.
	WindowDays int
	// CostPerGB prices the findings.
	CostPerGB float64
	// OutDir receives all generated files.
	OutDir string
	// MCPURL, when set, enables the experimental MCP findings hook.
	MCPURL string
}

// FromEnv builds the defaults layer from the environment.
func FromEnv() Config {
	return Config{
		ClickHouseURL:      envOr("CLICKHOUSE_HTTP_URL", "http://localhost:8123"),
		ClickHouseUser:     os.Getenv("CLICKHOUSE_USER"),
		ClickHousePassword: os.Getenv("CLICKHOUSE_PASSWORD"),
		SignozURL:          envOr("SIGNOZ_API_URL", "http://localhost:8080"),
		SignozAPIKey:       os.Getenv("SIGNOZ_API_KEY"),
		WindowDays:         envInt("TELELENS_WINDOW_DAYS", 7),
		CostPerGB:          envFloat("COST_PER_GB", analyze.DefaultCostPerGB),
		OutDir:             envOr("TELELENS_OUT_DIR", "out"),
		MCPURL:             os.Getenv("TELELENS_MCP_URL"),
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			return f
		}
	}
	return def
}
