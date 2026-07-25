# Contributing to TELELENS

Thanks for your interest. TELELENS is a single Go binary with one loud invariant. Contributions that respect it are very welcome.

## Development setup

```bash
git clone https://github.com/vinayaksonthalia/telelens && cd telelens
go build ./... && go vet ./... && go test ./...   # 41 tests, 7 packages — must stay green
./telelens scan --fixtures                        # full offline path, no instance needed
```

Live-mode development needs a self-hosted SigNoz with ClickHouse reachable — `DOCS.md` covers it.

## Ground rules

- **READ-ONLY is the product.** TELELENS issues `SELECT`/`GET` only, never applies anything, and every generated fix ships for human review (the drop-metrics block is committed commented-out under a review banner). A change that mutates a user's system will not be merged, however convenient.
- **The safety refusal is non-negotiable**: a sampling policy that would lose one error trace exits non-zero. `TestSimulateSafetyInvariant` pins this — keep it green.
- **Numbers carry their counterweights.** The measured −95% never appears without its ~6% error-storm caveat; keep every claim paired with the evidence file that backs it.
- CLI output follows the house style: severity semantics, `Error:`/`Why:`/`Try:` on failures, `NO_COLOR` respected, `--json` stays stdout-pure.
- Add tests with behavior changes; prefer boring, readable Go.

## Filing issues

Include the command, the findings/summary output (scrub hostnames if needed), your SigNoz/ClickHouse versions, and whether the scan was fixtures or live.

Licensed under MIT; by contributing you agree your work is too.
