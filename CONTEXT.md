# Domain context

## Product

A weather-aware travel and daily planning assistant, implemented as a Go Agent runtime plus a Web workspace. It combines live tools, private travel documents, user-confirmed constraints and traceable evidence to propose useful plans and alternatives.

Current implementation: CLI, calculator, datetime, QWeather tool, complete-turn buffer memory, and a local Vue/TypeScript trip workspace backed by Go HTTP and SQLite. The workspace persists editable constraints and complete successful turns, isolates browser owners, and supports durable SSE run events, idempotent starts and cancellation. An explicitly labelled deterministic demo requires no external services. RAG, token budgeting/summaries, maps and itinerary generation remain planned. See `docs/trip-workspace.md` and `docs/adr/0001-local-trip-workspace.md` for the current single-process, local-only boundary.

The weather tool accepts an optional destination forecast date, checks membership in the actual returned dates, and reports unknown when absent. Forecast text includes provider/location and retrieval/update timestamps; this is not yet a structured weather card or freshness guarantee. The generic precipitation-based activity hints below do not provide personalised itineraries. See `docs/weather-date-coverage.md` for the bounded Issue #4 slice and the existing provider API migration dependency.

The current-observation path also returns provider/location, retrieval time, API update time, observation time and observation age. Missing, invalid or future observation timestamps do not become fresh evidence; no universal weather expiry threshold is imposed. This is the bounded Issue #14 slice, with fixed HTTP tests rather than a live provider verification.

## Vocabulary

The OpenAI-compatible client rejects serialized request bodies over 1 MiB before transport, including tool schemas, system constraints and tool-loop growth. This fixed UTF-8/JSON byte boundary preserves complete message/tool pairs by failing the turn instead of truncating it. It is not a token budget, provider context guarantee or a cap on serialization memory; automatic summarisation and pruning remain planned.

The Web run record and terminal event now expose this local request-size error with byte counts and recovery advice. Only this typed local error is public; provider and wrapper errors remain hidden. An oversized later request does not undo earlier model/tool calls, and a failed turn leaves successful history unchanged.

Real-model runs record initial history-window evidence in context.ready: loaded versus saved message/complete-turn counts and omitted messages. The event describes the first model request's retained history, excluding the current input and system constraints; later tool-loop growth is not a token budget. Full persisted history remains separate, and the newest complete tool turn may exceed the soft message limit. Demo mode does not pretend to call a model.

Forecast output now includes per-date precipitation and application-labelled indoor activity categories when the amount is valid and the provider update is within a six-hour planning window. Missing/invalid/future/older evidence produces no activity recommendation; zero precipitation does not imply outdoor suitability. These are generic rules, not personalised venue choices or itinerary edits, and remain separate from provider observations.

Activity planning also checks the forecast date against the destination's local date at retrieval, using the selected city's IANA timezone. Past dates keep their raw forecast record but produce no new activity hint; missing or invalid timezone data also prevents hints. The machine timezone and the provider update's numeric offset do not identify the destination. Embedded standard-library tzdata supports standalone binaries; this is not a historical-weather API.

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

The Web workspace can create a fresh conversation from the current saved constraints using the existing session-creation API. The original session/history and page-local draft remain intact; no messages, run state or draft are copied. Unsaved constraints, active execution and terminal refresh block this action. It resets the conversation context by creating a new session, not by summarising or deleting history.

The confirmed-constraint count reads the selected session's saved constraints, not the editable form. Draft edits and failed saves leave the count unchanged; a successful save replaces it with the returned persisted state. The UI explicitly labels the saved-only count and warns when the form has unsaved changes. This is a completeness indicator, not a validation of itinerary feasibility.

Session creation/update and run-start JSON bodies must be single objects with unique members within each object, including escaped and case-insensitive aliases matching Go struct fields. Duplicate fields return HTTP 400 before persistence or execution; unknown fields, types and the 32 KiB limit remain enforced. The uniqueness rule currently assumes struct-shaped request objects, not arbitrary case-sensitive user dictionaries.

The page requests the browser's native leave confirmation while constraints are unsaved or any trip has an unsent nonblank message draft. Current input overrides its cached draft entry; saved/sent/cleared state does not warn unnecessarily. Accepting navigation still discards page-local drafts. This is best-effort beforeunload protection, not draft persistence or crash/mobile process recovery; browser policies may suppress the dialog.
