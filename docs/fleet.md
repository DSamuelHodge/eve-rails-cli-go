# Building A Fleet

Eve Rails CLI is most useful when several agents share the same catalog of
tools, skills, evals, approvals, channels, schedules, and memory conventions.

## Fleet Shape

`manifests/agents.yml` names each agent and the reusable pieces it uses:

```yaml
defaults:
  model: openai/gpt-5.5
  owner: agent-platform
  channels: [slack, telegram]   # platform presets; no catalog entry needed
  schedules: []
  evals: [standard]
  tools:                        # every agent inherits these catalog tools
    search_customers: 1.0.0
  skills:                       # every agent inherits these catalog skills
    triage_customer_issue: 1.0.0
  memory:                       # every agent inherits these catalog memories
    customer_profile: 1.0.0

agents:
  - name: support
    responsibility: Triage and resolve support requests.
    tools:
      prepare_refund: 1.0.0
    approvals:
      prepare_refund: required
    subagents:
      - name: triage
        title: Triage Rep
        role_id: triage
        responsibility: Classify and route tickets.
        tools: [search_customers]
        skills: [triage_customer_issue]
```

`manifests/catalog.yml` defines the reusable pieces once:

```yaml
tools:
  search_customers:
    version: 1.0.0
    side_effects: read
  prepare_refund:
    version: 1.0.0
    side_effects: money
skills:
  triage_customer_issue:
    version: 1.0.0
evals:
  standard:
    version: 1.0.0
approvals:
  required:
    version: 1.0.0
memory:
  customer_profile:
    version: 1.0.0
    retention: 180d
```

## Defaults, Shared, And Per-Agent

Component resolution merges three layers in order: `defaults` first, then
`shared`, then per-agent values. Later layers override earlier ones on version
conflicts, so a fleet-wide default tool can be pinned differently for one
agent.

Subagents author their own slots: a subagent's `tools`, `skills`, and `memory`
render as real files under `agent/subagents/<name>/` rather than prose, because
Eve subagents inherit nothing from the root agent.

## Built-in Harness Tools

Eve gives every agent a default harness for free: `bash`, `read_file`,
`write_file`, `glob`, `grep`, `web_fetch`, `web_search`, `todo`, and
`ask_question`, plus `agent`, `load_skill`, and `connection_search` where
applicable. eve-rails never generates or disables them — they are implicit and
shared by all agents and subagents. Define catalog tools only for capabilities
beyond the harness.

## Implicit Channel Presets

`slack`, `telegram`, `discord`, `teams`, `twilio`, `linear`, `github`, and
`eve` are platform presets. Listing one in `defaults.channels` or an agent's
`channels` generates a working `agent/channels/<slug>.ts` without a
`catalog.yml` entry, using platform defaults (`connectSlackCredentials(
"slack/my-agent")` for Slack, bot webhooks for Telegram, and so on). Define the
channel in the catalog to override defaults.

Channels are root-only in Eve. A declared subagent never owns channels, and
`eve-rails doctor` warns when a subagent declares them.

## Money And Approval

Tools with side effects such as `write`, `external`, `money`, or `production`
should have approval coverage before deployment. `doctor` checks this so risky
tools do not quietly ship without a policy.

Approval policies declare whether they block, so semantics live in data rather
than a name or prose:

```yaml
# manifests/catalog.yml
approvals:
  required:
    version: 1.0.0
    blocking: true    # approval: always()
  audit_log:
    version: 1.0.0
    blocking: false   # approval: never() - auto-approve and log
```

Every rendered `agent/tools/<tool>.ts` carries a real Eve approval gate
(`approval: always()` / `approval: never()`) imported from
`eve/tools/approval`. The gate is resolved from the manifest's per-agent
approval-policy assignment first, then the catalog tool's `required_approvals`,
then the side-effect class. A policy assigned without `blocking` defaults to
`always()` and `doctor` warns, so a `*-logged` policy cannot silently mean
"auto-approve" while its description says otherwise.

`eve-rails doctor --templates` also runs a `rendered-approval-gates` check that
re-renders each tool and asserts the gate in the output matches the manifest's
approval-policy assignment.

Sandbox boundaries in `x_runtime_policy.sandbox` render a real `agent/sandbox.ts`
with a network policy: `read-only` and `workspace-write` map to `deny-all`;
`network-read`, `external-write-gated`, `production-write-gated`, and
`approval-gated` map to `allow-all` and rely on the rendered approval gates.
With no boundary configured, Eve's default sandbox applies and no file is
generated.

```sh
eve-rails doctor --all
eve-rails apply manifests/agents.yml
eve-rails render --all --check
```

Use `eve-rails --help` and `eve-rails <command> --help` for the current
command flags.
