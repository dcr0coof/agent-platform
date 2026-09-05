# Domain context

## Product

A weather-aware travel and daily planning assistant, implemented as a Go Agent runtime plus a Web workspace. It combines live tools, private travel documents, user-confirmed constraints and traceable evidence to propose useful plans and alternatives.

Current implementation: CLI, calculator, datetime, QWeather tool, complete-turn buffer memory, regression tests. Web, RAG, persistence, maps and itinerary generation are planned, not shipped.

## Vocabulary

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
