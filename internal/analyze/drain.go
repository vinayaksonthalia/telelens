package analyze

import (
	"sort"
	"strings"
)

// Drain-style log template mining (simplified from He et al., "Drain: An
// Online Log Parsing Approach with Fixed Depth Tree"). Lines are tokenized,
// bucketed by token count (the fixed-depth part), and clustered by token
// similarity; positions that vary across members become the wildcard <*>.
//
// This runs locally over a bounded sample of bodies (NFR-7): raw log payloads
// never leave the process.

// Template is one mined log template with its aggregate volume statistics.
type Template struct {
	// Text is the template with variable positions replaced by <*>.
	Text string
	// Service the template was mined from.
	Service string
	// Severity is the dominant severity of the cluster.
	Severity string
	// Count is the (weighted) number of records matching the template.
	Count int64
	// Bytes is the (weighted) total body bytes of matching records.
	Bytes int64
	// Exact is true when no position varied: every record is an identical
	// duplicate (dedup candidate rather than sampling candidate).
	Exact bool
}

// DrainConfig tunes the miner.
type DrainConfig struct {
	// SimilarityThreshold in [0,1]: minimum fraction of equal tokens for a
	// line to join an existing cluster. Drain's customary default is 0.5.
	SimilarityThreshold float64
	// MaxTokens caps tokenization; longer bodies are truncated (jumbo bodies
	// still cluster by their prefix).
	MaxTokens int
}

// DefaultDrainConfig matches the values validated on the fixture corpus.
func DefaultDrainConfig() DrainConfig {
	return DrainConfig{SimilarityThreshold: 0.5, MaxTokens: 64}
}

// LogRecord is the miner's input.
type LogRecord struct {
	Service  string
	Severity string
	Body     string
	// Weight is how many stored records this sample represents (>=1).
	Weight int64
}

type cluster struct {
	tokens   []string // "<*>" marks wildcard positions
	service  string
	severity string
	count    int64
	bytes    int64
	exact    bool
}

// MineTemplates clusters records into templates, per (service, token-count)
// bucket, and returns templates sorted by Bytes descending (cost order).
func MineTemplates(records []LogRecord, cfg DrainConfig) []Template {
	if cfg.SimilarityThreshold <= 0 || cfg.SimilarityThreshold > 1 {
		cfg.SimilarityThreshold = 0.5
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 64
	}

	type bucketKey struct {
		service string
		nTokens int
	}
	buckets := map[bucketKey][]*cluster{}

	for _, r := range records {
		w := r.Weight
		if w < 1 {
			w = 1
		}
		tokens := strings.Fields(r.Body)
		if len(tokens) == 0 {
			continue
		}
		if len(tokens) > cfg.MaxTokens {
			tokens = tokens[:cfg.MaxTokens]
		}
		key := bucketKey{r.Service, len(tokens)}
		var best *cluster
		bestSim := cfg.SimilarityThreshold
		for _, c := range buckets[key] {
			if sim := similarity(c.tokens, tokens); sim >= bestSim {
				best, bestSim = c, sim
			}
		}
		bodyBytes := int64(len(r.Body)) * w
		if best == nil {
			buckets[key] = append(buckets[key], &cluster{
				tokens:   append([]string(nil), tokens...),
				service:  r.Service,
				severity: r.Severity,
				count:    w,
				bytes:    bodyBytes,
				exact:    true,
			})
			continue
		}
		// Merge: differing positions become wildcards.
		for i := range best.tokens {
			if best.tokens[i] != tokens[i] {
				if best.tokens[i] != wildcard {
					best.tokens[i] = wildcard
				}
				best.exact = false
			}
		}
		best.count += w
		best.bytes += bodyBytes
	}

	var out []Template
	for _, cs := range buckets {
		for _, c := range cs {
			out = append(out, Template{
				Text:     strings.Join(c.tokens, " "),
				Service:  c.service,
				Severity: c.severity,
				Count:    c.count,
				Bytes:    c.bytes,
				Exact:    c.exact,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bytes != out[j].Bytes {
			return out[i].Bytes > out[j].Bytes
		}
		return out[i].Text < out[j].Text
	})
	return out
}

const wildcard = "<*>"

// similarity is the fraction of positions where the cluster token equals the
// candidate token; existing wildcards count as matches (Drain's simSeq).
func similarity(clusterTokens, tokens []string) float64 {
	if len(clusterTokens) != len(tokens) {
		return 0
	}
	eq := 0
	for i := range tokens {
		if clusterTokens[i] == tokens[i] || clusterTokens[i] == wildcard {
			eq++
		}
	}
	return float64(eq) / float64(len(tokens))
}
