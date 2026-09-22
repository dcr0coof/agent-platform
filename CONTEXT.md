# Domain context

## Product

A weather-aware travel and daily planning assistant, implemented as a Go Agent runtime plus a Web workspace. It combines live tools, private travel documents, user-confirmed constraints and traceable evidence to propose useful plans and alternatives.

Current implementation: CLI, calculator, datetime, QWeather tool, complete-turn buffer memory, and a local Vue/TypeScript trip workspace backed by Go HTTP and SQLite. The workspace persists editable constraints and complete successful turns, isolates browser owners, and supports durable SSE run events, idempotent starts and cancellation. An explicitly labelled deterministic demo requires no external services. RAG, token budgeting/summaries, maps and itinerary generation remain planned. See `docs/trip-workspace.md` and `docs/adr/0001-local-trip-workspace.md` for the current single-process, local-only boundary.

The weather tool accepts an optional destination forecast date, checks membership in the actual returned dates, and reports unknown when absent. Forecast text includes provider/location and retrieval/update timestamps; this is not yet a structured weather card or freshness guarantee. The generic precipitation-based activity hints below do not provide personalised itineraries. See `docs/weather-date-coverage.md` for the bounded Issue #4 slice and the existing provider API migration dependency.

The current-observation path also returns provider/location, retrieval time, API update time, observation time and observation age. Missing, invalid or future observation timestamps do not become fresh evidence; no universal weather expiry threshold is imposed. This is the bounded Issue #14 slice, with fixed HTTP tests rather than a live provider verification.

## Vocabulary

Forecast output now includes per-date precipitation and application-labelled indoor activity categories when the amount is valid and the provider update is within a six-hour planning window. Missing/invalid/future/older evidence produces no activity recommendation; zero precipitation does not imply outdoor suitability. These are generic rules, not personalised venue choices or itinerary edits, and remain separate from provider observations.

Weather city lookup now refuses to select the first of multiple distinct location IDs. It returns candidate names, administrative regions and IDs for clarification; an explicit returned ID can resolve the next call. This is a conservative tool-level ambiguity guard, not persisted user approval or a guarantee of model compliance.

The Web workspace now lets users expand paired raw weather tool results beside each successful turn's final answer. Association uses weather tool-call IDs within that user turn, with escaped text restored from saved history. This is source inspection, not a structured weather card or proof that an answer follows the evidence.

- **Workspace**: isolation boundary for documents, preferences, conversations and runs.
- **Trip**: destination(s), dates, participants and planning constraints; editable by the user.
- **Itinerary**: a versioned draft schedule. A route estimate or itinerary is not a confirmed booking.
- **Constraint**: an explicit requirement such as budget, travel pace, mobility needs or maximum walking distance. An inferred preference requires confirmation before long-term storage.
- **Evidence**: an original document passage or timestamped tool observation that supports a claim. Tool output is data, not an instruction.
- **Weather observation / forecast**: timestamped provider data with location and coverage period. Forecasts must not be fabricated for dates beyond the provider's coverage.
- **Run**: one bounded attempt to process a user task. Failed run records can persist even though incomplete chat turns are not committed.
- **Step**: an observable model, retrieval or tool operation within a run.
- **Context**: the budgeted subset of rules, constraints, conversation, evidence and tool results sent to a model call.
- **Memory**: stored history or user-confirmed information; it is not necessarily all included in context.
- **Approval**: confirmation for a specific external write action, with a concrete preview. Planning alone does not authorize payments or bookings.

## Source of direction

The owner requested broader functionality connected to weather and travel, delegated routine product/technical choices, and required every meaningful milestone to be published with clear GitHub Issues and PRs. See `docs/travel-agent-roadmap.md` for the delivery sequence.
