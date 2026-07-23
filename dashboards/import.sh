#!/usr/bin/env bash
# Import the TELELENS "Telemetry Bill" dashboard pack + guardrail alert rules
# into any SigNoz instance. Nothing here is machine-specific.
#
# Usage:
#   export SIGNOZ_URL=http://localhost:8080        # no trailing slash
#   export SIGNOZ_API_KEY=<api key>                # Settings -> API Keys
#   export SIGNOZ_ALERT_CHANNEL=my-slack           # optional; see below
#   ./import.sh [--dashboards-only|--alerts-only]
#
# Alert rules ship with a TELELENS_ALERT_CHANNEL placeholder (SigNoz v2 rules
# require >=1 notification channel). At import time it is replaced by:
#   1. $SIGNOZ_ALERT_CHANNEL if set, else
#   2. the first existing channel from GET /api/v1/channels, else
#   3. the alert import fails with instructions.
#
# The API key is only ever sent as the SIGNOZ-API-KEY header.

set -euo pipefail

: "${SIGNOZ_URL:?Set SIGNOZ_URL, e.g. http://localhost:8080}"
: "${SIGNOZ_API_KEY:?Set SIGNOZ_API_KEY (SigNoz Settings -> API Keys)}"

SIGNOZ_URL="${SIGNOZ_URL%/}"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODE="${1:-all}"

post() {
  local endpoint="$1" payload="$2" label="$3" http_code
  http_code=$(curl -sS -o /tmp/telelens-import-resp.$$ -w '%{http_code}' \
    -X POST "${SIGNOZ_URL}${endpoint}" \
    -H "Content-Type: application/json" \
    -H "SIGNOZ-API-KEY: ${SIGNOZ_API_KEY}" \
    --data-binary "${payload}")
  if [[ "$http_code" =~ ^2 ]]; then
    echo "  OK   ($http_code) $label"
  else
    echo "  FAIL ($http_code) $label" >&2
    sed 's/^/         /' /tmp/telelens-import-resp.$$ >&2 || true
    rm -f /tmp/telelens-import-resp.$$
    return 1
  fi
  rm -f /tmp/telelens-import-resp.$$
}

if [[ "$MODE" != "--alerts-only" ]]; then
  echo "Importing Telemetry Bill dashboards..."
  for f in cost-overview cardinality-explorer savings-tracker; do
    post "/api/v1/dashboards" "@$DIR/$f.json" "$f"
  done
fi

if [[ "$MODE" != "--dashboards-only" ]]; then
  echo "Resolving alert notification channel..."
  CHANNEL="${SIGNOZ_ALERT_CHANNEL:-}"
  if [[ -z "$CHANNEL" ]]; then
    CHANNEL=$(curl -sS "${SIGNOZ_URL}/api/v1/channels" -H "SIGNOZ-API-KEY: ${SIGNOZ_API_KEY}" \
      | python3 -c "import json,sys; d=json.load(sys.stdin); ch=d.get('data') or []; print(ch[0]['name'] if ch else '')")
  fi
  if [[ -z "$CHANNEL" ]]; then
    echo "  FAIL: no notification channel found. Create one (Settings -> Alert Channels)" >&2
    echo "        or set SIGNOZ_ALERT_CHANNEL, then re-run with --alerts-only." >&2
    exit 1
  fi
  echo "  using channel: $CHANNEL"
  echo "Importing guardrail alert rules..."
  python3 - "$DIR/../alerts/guardrails.json" "$CHANNEL" <<'PYEOF' > /tmp/telelens-rules.$$
import json, sys
rules = json.load(open(sys.argv[1]))
out = json.dumps(rules).replace("TELELENS_ALERT_CHANNEL", sys.argv[2])
print(out)
PYEOF
  COUNT=$(python3 -c "import json; print(len(json.load(open('/tmp/telelens-rules.$$'))))")
  for i in $(seq 0 $((COUNT-1))); do
    NAME=$(python3 -c "import json; print(json.load(open('/tmp/telelens-rules.$$'))[$i]['alert'])")
    post "/api/v2/rules" "$(python3 -c "import json; print(json.dumps(json.load(open('/tmp/telelens-rules.$$'))[$i]))")" "$NAME"
  done
  rm -f /tmp/telelens-rules.$$
fi

echo "Done."
