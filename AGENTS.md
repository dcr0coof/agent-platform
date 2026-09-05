# Repository working agreement

## User-authorized workflow

- This project targets AI application / Agent development portfolio roles. The current product direction is a weather-aware travel and daily planning assistant with evidence-backed retrieval and explicit context management.
- The owner requested GitHub tracking for every meaningful change. Before implementing a feature, create or reuse a GitHub Issue with scope, acceptance criteria and dependencies. Use a `codex/` feature branch; keep commits cohesive and push each completed, verified milestone.
- Create or update one PR for each independent change. Describe the problem, resulting behavior, implementation decisions, validation and limitations; link the real Issue number with `Closes #N` when fully addressed. Never invent GitHub links or issue numbers.
- Use normal PR review and required CI checks. Do not force-push, bypass checks, or change repository visibility. Report local-only commits honestly when authentication or network blocks publishing.
- The owner authorized routine local changes and publishing project code, Issues and PRs to `dcr0coof/agent-platform`. Do not repeatedly ask for the same authorization. Account login must be completed through authorized credentials or official login; never request or store account passwords.
- Keep credentials out of source, remote URLs, commands, logs, screenshots, Issues and PRs. Read-only status output must redact remote user information.
- If publishing is blocked, save reviewable Issue/PR drafts locally and report the exact blocker; do not treat drafts as published work.

## Verification

- Run `go test ./...`, `go vet ./...`, and `go build ./...` for Go changes. Run `go test -race ./...` when supported; Linux CI covers the race detector.
- Preserve existing complete-turn memory behavior and tool-call/result pairing. Run events and successful chat history are separate concepts.
- No real purchases, bookings or outgoing messages are implied by the planning assistant. Add concrete user approval before external write actions.

## Agent skills

### Issue tracker

GitHub Issues and Pull Requests in `dcr0coof/agent-platform`. See `docs/agents/issue-tracker.md`.

### Triage labels

Use existing repository labels when available, with explicit category and triage-state mapping. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context project. See `CONTEXT.md` and `docs/agents/domain.md`; architecture decisions are recorded under `docs/adr/` when established.
