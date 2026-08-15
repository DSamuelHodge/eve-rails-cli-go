#!/usr/bin/env bash
set -euo pipefail
trap 'echo "CLI acceptance failed near line ${LINENO}: ${BASH_COMMAND}" >&2' ERR

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT/bin/eve-rails"

cd "$ROOT"

mkdir -p bin
go build -o "$BIN" .

echo "== help =="
set -x
"$BIN" --help >/dev/null
for cmd in init wizard plan apply render doctor generate outdated update hotload deploy eval test preview migrate rollback inspect graph; do
  "$BIN" "$cmd" --help >/dev/null
done
for sub in agent tool skill subagent channel schedule approval eval memory batch migration; do
  "$BIN" generate "$sub" --help >/dev/null
done
set +x

echo "== temp project commands =="
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

"$BIN" init "$TMP/demo-json" --template basic --model openai/gpt-5.5 --owner cli-test --yes --dry-run --json >/dev/null
"$BIN" init "$TMP/demo" --template basic --model openai/gpt-5.5 --owner cli-test --yes >/dev/null
(
  cd "$TMP/demo"
  export TWILIO_ACCOUNT_SID=AC00000000000000000000000000000000
  export TWILIO_AUTH_TOKEN=test-token
  export TWILIO_FROM_NUMBER=+15557654321
  "$BIN" generate tool refund_customer --side-effects money --dry-run --json >/dev/null
  "$BIN" generate tool refund_customer --side-effects money --json >/dev/null
  "$BIN" generate skill handle_refund --json >/dev/null
  "$BIN" generate subagent researcher --json >/dev/null
  "$BIN" generate channel slack --kind slack --connect-uid slack/support-agent --json >/dev/null
  "$BIN" generate channel sms_support --kind twilio --allow-from "+15551234567" --messaging-from env:TWILIO_FROM_NUMBER --json >/dev/null
  "$BIN" generate schedule weekday_triage --schedule "0 9 * * 1-5" --json >/dev/null
  "$BIN" generate approval refund_customer --json >/dev/null
  "$BIN" generate eval refund_policy --json >/dev/null
  "$BIN" generate memory customer_profile --retention 180d --json >/dev/null
  "$BIN" generate migration customer_profile_v2 --json >/dev/null
  "$BIN" generate agent support \
    --owner cli-test \
    --model openai/gpt-5.5 \
    --description "Support smoke agent." \
    --with-tools refund_customer \
    --with-skills handle_refund \
    --with-subagents researcher \
    --with-channels slack,sms_support \
    --with-schedules weekday_triage \
    --with-evals refund_policy \
    --with-memory customer_profile \
    --approval required \
    --auth http-basic-env \
    --visibility internal \
    --cost-budget 10 \
    --token-budget 100000 \
    --timeout 30s \
    --json >/dev/null
  "$BIN" plan --json >/dev/null
  "$BIN" generate batch --dry-run --json >/dev/null
  "$BIN" apply --json >/dev/null
  "$BIN" render --all --check >/dev/null
  "$BIN" doctor --all --templates --updates --env production --connections --budgets >/dev/null
  "$BIN" doctor --all --templates --updates --fix --dry-run >/dev/null
  "$BIN" outdated --json >/dev/null
  "$BIN" update --agent support --minor --plan --json >/dev/null
  "$BIN" hotload --agent support --current 1.0.0 skill:handle_refund@1.0.1 --json >/dev/null
  "$BIN" deploy --agent support --env production --require-evals --require-doctor --require-approvals --dry-run --json >/dev/null
  "$BIN" eval --agent support --dry-run --json >/dev/null
  "$BIN" test --agent support --dry-run --json >/dev/null
  "$BIN" preview --agent support --dry-run --json >/dev/null
  "$BIN" migrate --agent support --env production --dry-run --json >/dev/null
  "$BIN" rollback --agent support --to 1.0.0 --json >/dev/null
  "$BIN" inspect --agent support --json >/dev/null
  "$BIN" graph --all --format json >/dev/null
)

echo "== defaults, presets, and subagent files =="
TMP2="$(mktemp -d)"
trap 'rm -rf "$TMP" "$TMP2"' EXIT

mkdir -p "$TMP2/manifests"
cat > "$TMP2/manifests/catalog.yml" <<'CATALOG'
tools:
  search_customers:
    version: 1.0.0
    side_effects: read
  prepare_refund:
    version: 1.0.0
    side_effects: money
skills:
  summarize:
    version: 1.0.0
evals:
approvals:
  required:
    version: 1.0.0
    blocking: true
memory:
  customer_profile:
    version: 1.0.0
    retention: 180d
channels:
schedules:
CATALOG
cat > "$TMP2/manifests/agents.yml" <<'AGENTS'
defaults:
  model: openai/gpt-5.5
  owner: cli-test
  channels: [slack, telegram]
  schedules: []
  evals: []
  tools:
    search_customers: 1.0.0
  skills:
    summarize: 1.0.0

agents:
  - name: support
    version: 1.0.0
    responsibility: Triage and resolve support requests.
    tools:
      prepare_refund: 1.0.0
    approvals:
      prepare_refund: required
    x_runtime_policy:
      sandbox: approval-gated
    subagents:
      - name: triage
        title: Triage Rep
        role_id: triage
        responsibility: Classify and route tickets.
        tools: [search_customers]
        skills: [summarize]
        memory: [customer_profile]
AGENTS
(
  cd "$TMP2"
  "$BIN" doctor --manifest manifests/agents.yml --catalog manifests/catalog.yml >/dev/null
  "$BIN" apply manifests/agents.yml --catalog manifests/catalog.yml --template-dir "$ROOT/templates/agent" >/dev/null
  "$BIN" doctor --all --manifest manifests/agents.yml --catalog manifests/catalog.yml --template-dir "$ROOT/templates/agent" >/dev/null
  test -f agents/support/agent/channels/slack.ts
  test -f agents/support/agent/channels/telegram.ts
  test -f agents/support/agent/tools/search_customers.ts
  test -f agents/support/agent/skills/summarize.md
  test -f agents/support/agent/subagents/triage/tools/search_customers.ts
  test -f agents/support/agent/subagents/triage/skills/summarize.md
  test -f agents/support/agent/subagents/triage/memory/customer_profile.ts
  grep -q "connectSlackCredentials" agents/support/agent/channels/slack.ts
  grep -q "telegramChannel" agents/support/agent/channels/telegram.ts
  grep -q "Declared channels" agents/support/agent/subagents/triage/instructions.md && exit 1 || true
  grep -q "approval: always()" agents/support/agent/tools/prepare_refund.ts
  grep -q 'from "eve/tools/approval"' agents/support/agent/tools/prepare_refund.ts
  grep -q "approval: never()" agents/support/agent/tools/search_customers.ts
  test -f agents/support/agent/sandbox.ts
  grep -q 'networkPolicy: "allow-all"' agents/support/agent/sandbox.ts
  "$BIN" render --all --check --template-dir "$ROOT/templates/agent" >/dev/null
)

echo "CLI acceptance sweep passed."
