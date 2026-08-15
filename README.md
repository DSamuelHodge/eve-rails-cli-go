<div align="center">
  <a href="https://github.com/vercel/eve">
    <img alt="eve logo" src="https://raw.githubusercontent.com/vercel/eve/main/.github/assets/eve.svg" height="96">
  </a>
  <h1>Eve Rails CLI</h1>

[![CI/CD](https://github.com/DSamuelHodge/eve-rails-cli-go/actions/workflows/ci.yml/badge.svg)](https://github.com/DSamuelHodge/eve-rails-cli-go/actions/workflows/ci.yml)
[![Version](https://img.shields.io/badge/version-0.1.1-blue.svg)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8.svg)](https://go.dev/)
[![Node](https://img.shields.io/badge/Node.js-24%2B-339933.svg)](https://nodejs.org/)
[![Eve](https://img.shields.io/badge/Eve-0.24%2B-black.svg)](https://www.npmjs.com/package/eve)

**Eve gives you the agent. Eve Rails CLI gives you the org chart.**

</div>

[Eve](https://github.com/vercel/eve) is Vercel's filesystem-first framework for durable AI
agents — instructions, tools, skills, channels, and schedules as conventional folders that
compile into a running agent. That's the right unit for one agent, or ten, built by hand with
care.

Eve Rails CLI is what you reach for when "one agent" becomes "a department," and "a
department" becomes "a fleet."

## The problem this solves

The tenth agent stops looking like the first one. Different approval conventions. Different
sandbox assumptions. Nobody can say with confidence who's accountable when agent #340 does
something wrong, because agent #340 was hand-built on a Tuesday by whoever was free.

Fleet-scale agent operations need what every fleet-scale system needs: **a schema, a
compiler, and a doctor.** You describe your organization the way you'd describe it to a new
hire — departments, ownership, who delegates to whom, who's accountable for what outcome — in
one set of YAML manifests. Eve Rails CLI compiles that into real, running Eve agents: correct
directory structure, correct approval gates, correct sandbox boundaries, every time, for every
agent, whether you're standing up 4 or 4,000.

## Why topology is the whole point

This isn't "generate a lot of agents fast." It's that a fleet without topology is just a pile
of scripts that happen to call themselves agents. Real organizations have departments, each
owning a clear outcome, each delegating narrow execution work to people who are accountable
for that specific piece. The manifest makes that structure explicit instead of implicit:

- **Parent agents own outcomes, not just activity.** A parent's `responsibility:` is the SLA
  and the completion criteria, not just "route the lead." The parent is accountable even when
  a delegate made the mistake — the same way a manager is accountable for their team's output.
- **Delegates own narrow, checkable jobs.** No single agent holds "dedupe, enrich, score, and
  route" in its head at once. Each delegate does one thing, against a stated rule, and hands
  off cleanly.
- **Approval policies trace to the actor, not a shared bucket.** When something goes wrong,
  "which agent did this" resolves to a name — a specific department, a specific delegate —
  not a fleet-wide shrug.

This is the same reason Rails conventions exist for web apps: not because uniformity is
pretty, but because uniformity is what makes a system *auditable* at a size where no one
person can hold the whole thing in their head anymore.

## Quick start

### 1. Install

```sh
git clone https://github.com/DSamuelHodge/eve-rails-cli-go.git
cd eve-rails-cli-go
go install github.com/DSamuelHodge/eve-rails-cli-go@latest
```

The module path still ends in `eve-rails-cli-go`, so the installed binary keeps that name;
rename it once to match the short command used below:

```sh
mv "$(go env GOPATH)/bin/eve-rails-cli-go" "$(go env GOPATH)/bin/eve-rails"
```

This walkthrough builds a small fleet, not a single agent — that's the point. You'll end up
with two departments (`support` and `billing`) sharing one catalog, with `billing` gated
behind an approval because it touches money.

### 2. Create the fleet

`init` scaffolds a project with empty manifests: `manifests/agents.yml` (which agents exist),
`manifests/catalog.yml` (reusable tools/skills/approvals), and `manifests/environments.yml`.
Nothing is agent-specific yet — this is the fleet's foundation, not one agent's home.

```sh
eve-rails init my-project --yes
cd my-project
```

### 3. Add reusable pieces to the catalog

Define the tools and skills once, before attaching them to anyone. `search_customers` is a
plain read; `prepare_refund` moves money, so it's tagged accordingly:

```sh
eve-rails generate tool search_customers --side-effects read
eve-rails generate tool prepare_refund --side-effects money
eve-rails generate skill triage_customer_issue
```

`generate` writes each of these into `manifests/catalog.yml`. Nothing is rendered yet — the
catalog is just inventory, shared by every department that ends up using it.

### 4. Add two departments that share the catalog

`support` only reads customer data, so it needs no approval gate. `billing` uses the same
read tool plus the money-moving one, so its refund tool is gated behind the `required`
approval policy that `init` already scaffolded into the catalog:

```sh
eve-rails generate agent support \
  --with-tools search_customers \
  --with-skills triage_customer_issue

eve-rails generate agent billing \
  --with-tools search_customers,prepare_refund \
  --with-skills triage_customer_issue \
  --approval required
```

Now render both from the manifest in one pass:

```sh
eve-rails apply manifests/agents.yml
eve-rails doctor --all
```

`doctor` is what actually enforces the gate: if `billing` had `prepare_refund` without an
approval policy attached, `doctor` fails the build on `approval-coverage` instead of letting
a money-moving tool ship quietly. Try removing the `approvals:` line from `billing` in
`manifests/agents.yml` and re-running `doctor --all` to see that check fire.

### 5. What just got created

Nothing here is generated magic you can't inspect. `catalog.yml` now includes both tools and
the skill:

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
approvals:
  required:
    version: 1.0.0
```

And `agents.yml` connects those reusable pieces to each department:

```yaml
agents:
  - name: support
    tools:
      search_customers: 1.0.0
    skills:
      triage_customer_issue: 1.0.0

  - name: billing
    tools:
      search_customers: 1.0.0
      prepare_refund: 1.0.0
    skills:
      triage_customer_issue: 1.0.0
    approvals:
      prepare_refund: required
```

Edit the YAML by hand any time — run `eve-rails apply manifests/agents.yml` again after
manual edits to re-render generated agents. This is the whole trick at any scale: two
departments today, twenty tomorrow, all resolving the same tools and approvals from one
catalog instead of forty hand-copied decisions.

### 6. Verify the generated agents

```sh
cd agents/support
npm install
npm run typecheck
npm exec -- eve info --json
```

Repeat for `agents/billing`. For deeper fleet operations — hot-loading skill patches,
rolling back a bad deploy, inspecting the dependency graph across departments — see
[Building a fleet](docs/fleet.md).

### Interactive wizard

For a guided, terminal-first setup, run `eve-rails wizard` instead of typing individual
`init`/`generate` commands:

```sh
eve-rails wizard
```

The wizard walks through project init, adding reusable components, creating an agent, and
confirming an apply. It requires an interactive terminal; agents and CI should keep using
`init`, `generate`, `apply`, and the other flag-based commands (with `--json` for
machine-readable output). Running `wizard` without a TTY fails fast with that guidance.

## Requirements

- Go 1.24 or newer. Install Go from [go.dev/dl](https://go.dev/dl/).
- Node.js 24 or newer when testing generated Eve agents. The CI Eve smoke job uses Node 24
  because the Vercel Eve framework requires the modern Node runtime.
- Optional Vercel AI Gateway credentials for live model calls.

## Fleet model

Eve Rails CLI keeps fleet intent in YAML and renders one Eve project per agent:

```text
manifests/agents.yml   # fleet defaults and agent composition
manifests/catalog.yml  # reusable tools, skills, evals, approvals, channels
templates/agent/       # render templates
agents/<name>/         # generated Eve projects
```

The `support` and `billing` departments from the walkthrough above already show the shape of
`agents.yml` and `catalog.yml`. Two things worth naming that the walkthrough didn't need yet:

- **`defaults:`** sets fleet-wide fallbacks — model, owner, channels, schedules, evals — so
  you're not repeating them on every agent as the fleet grows past two departments.
- **`responsibility:`** is where the topology argument from earlier becomes concrete. It's
  not a hint for the model — it's the SLA text that says what "done" means for that
  department, which is what makes accountability traceable to a specific agent instead of the
  fleet as a whole.

```yaml
defaults:
  model: openai/gpt-5.5
  owner: agent-platform
  channels: []
  schedules: []
  evals: [standard]

agents:
  - name: support
    version: 1.0.0
    responsibility: Triage and resolve support requests.
    tools:
      search_customers: 1.0.0
    skills:
      triage_customer_issue: 1.0.0

  - name: billing
    version: 1.0.0
    responsibility: Answer billing questions and prepare safe refund actions.
    tools:
      search_customers: 1.0.0
      prepare_refund: 1.0.0
    skills:
      triage_customer_issue: 1.0.0
    approvals:
      prepare_refund: required
```

Tools, skills, evals, approvals, channels, schedules, memory, auth, and deployment metadata
can all be generated and reused the same way — every additional department resolves against
the same catalog instead of redefining its own copy. The catalog itself, once both tools from
the walkthrough are in it, looks like:

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
```

Run `eve-rails <command> --help` for the current options on any of these.

### Versioning: intent vs. truth

The fleet manifest keeps semver *intent* (`1.0.0`, `^1.0.0`, `~1.2.0`, or `catalog` to float
to the catalog version). Each generated agent separately ships a `versions.lock` that records
the *content digest* of every rendered component file:

```yaml
resolved:
  catalog/tools/search@1.0.0:
    source: manifests/catalog.yml
    digest: sha256:918abbf5…   # sha256 of the rendered tools/search.ts
```

The split matters: `outdated` and `update` work against the semver intent in the manifest, but
`doctor` and `inspect` compare rendered content digests — so if you change a catalog
component's content without bumping its version, that's flagged as stale rather than silently
slipping through. `rollback` restores `agent.manifest.yml` and `versions.lock` together, and a
restore is provably correct: the restored lock re-renders to the same digest.

## The doctor

`doctor` is what turns "we think this is fine" into "the build says this is fine." It checks,
before anything ships:

- **Approval coverage** — a tool with side effects like `write`, `external`, `money`, or
  `production` needs an explicit approval policy, or the build fails.
- **Memory retention** — a memory component with no stated retention policy fails the build,
  not a 2am incident.
- **Freshness** — catalog content that drifted from what's actually rendered gets flagged as
  stale.
- **Budgets and runtime config** — sandbox and budget settings are validated against what the
  agent is actually asking to do.

```sh
eve-rails doctor --all
```

`plan` shows you exactly what would be generated before anything touches disk — the same
discipline as `terraform plan`. `hotload` knows the difference between a skill patch (safe to
push live) and a tool change (needs a full redeploy) automatically, because the rule is
enforced in code, not remembered by whoever's on call.

## The Operator model

One operator, an entire fleet. Not one model doing the work of every department — that would
just rebuild the cognitive-overload problem the topology exists to solve, one layer up. An
operator model (Claude, Fable, GPT, or similar) directs Eve Rails CLI from a terminal or
agentic coding surface:

- **The fleet does the work.** Departments own outcomes. Delegates own narrow, checkable jobs.
  Every write traces to a specific accountable agent, regardless of who or what is directing
  the fleet from above.
- **The Operator runs the fleet.** It edits the manifest. It reads `plan` before `apply`. It
  reviews `doctor`'s failures instead of guessing. It treats a structural change to the fleet
  with the same discipline the fleet applies to any other risky action — nothing ships without
  passing the checks first.

The manifest is the interface between the two layers: the Operator's judgment goes in as YAML,
the fleet's accountable execution comes out as running agents, and `doctor` is the contract
that keeps the two honest with each other.

## Who this is for

- **Teams past the "a few agents" stage**, where hand-consistency has already started to slip
  and nobody wants to be the one who finds the inconsistency in production.
- **Anyone who needs to answer "who's accountable for this"** about an agent's action, not
  just "which agent ran."
- **Organizations that already think in departments** — support, sales, engineering, ops —
  and want their agent fleet to mirror the org chart they trust, instead of inventing a new
  mental model just for agents.

## Commands

Use `eve-rails --help` for the command list and `eve-rails <command> --help` for flags.

- Create: `init`, `wizard`, `generate`
- Render: `plan`, `apply`, `render`
- Validate: `doctor`, `test`, `eval`, `preview`
- Operate: `outdated`, `update`, `hotload`, `migrate`, `deploy`, `rollback`
- Inspect: `inspect`, `graph`

## Eve resources

- Eve repository: [github.com/vercel/eve](https://github.com/vercel/eve)
- Eve documentation: [eve.dev/docs](https://eve.dev/docs)
- Eve package: [npmjs.com/package/eve](https://www.npmjs.com/package/eve)
- Eve community: [GitHub Discussions](https://github.com/vercel/eve/discussions)

## License

Eve Rails CLI is available under the [MIT License](LICENSE). MIT is permissive and allows
private use, modification, distribution, sublicensing, and commercial use when the license
notice is preserved.

## Contributing

Contributions are welcome. See [CONTRIBUTING.md](.github/CONTRIBUTING.md) and the GitHub
issue and pull request templates.
