## Plan: Chat Ownership + Lifecycle Refactor (DRAFT)

This refactor makes Chat Service the only production chat API owner, removes overlapping Agent chat routing, and enforces strict conversation lifecycle semantics: clients create conversation first, then post messages only to existing conversation IDs, and unknown IDs return 404. It also fixes the persistence/metrics inconsistencies and the Mongo field-key mismatch that likely causes null message/pending-action reads. Error handling is tightened so provider/LLM failures return 5xx structured errors instead of silent fallback success, and deletion follows the hybrid policy (soft delete immediately + background hard purge). This aligns behavior with your bug findings while reducing architecture ambiguity and data integrity risk.

**Steps**
1. Clarify architecture boundary in code and docs: Chat owns user chat APIs; Agent HTTP chat path removed from routing in [internal/gateway/service.go](internal/gateway/service.go) and endpoint removal in [internal/agent/service.go](internal/agent/service.go); update service architecture in [docs/SERVICE_ARCHITECTURE.md](docs/SERVICE_ARCHITECTURE.md).
2. Introduce explicit conversation lifecycle APIs in [internal/chat/service.go](internal/chat/service.go): create conversation (server UUID only), delete conversation (hybrid delete), and chat health route moved here; wire docs in [docs/openapi/paths/chat.yaml](docs/openapi/paths/chat.yaml) and [docs/openapi/schemas/chat.yaml](docs/openapi/schemas/chat.yaml).
3. Enforce strict message contract in [internal/chat/service.go](internal/chat/service.go): POST messages requires existing conversation ID; if provided ID missing, return structured 404 using existing chat error shape.
4. Replace silent LLM fallback success path: in [internal/chat/service.go](internal/chat/service.go), on forwardToAgent/provider failure return structured 5xx error payload and increment error metrics consistently.
5. Fix persistence ordering and guarantees in chat flow: persist user message before provider call; ensure assistant write path is isolated; make write failures explicit and measurable instead of log-only.
6. Normalize Mongo field keys repository-wide to model bson tags in [internal/chat/repository.go](internal/chat/repository.go) and related models/indexes in [internal/shared/models/models.go](internal/shared/models/models.go), [internal/shared/database/mongo.go](internal/shared/database/mongo.go) to resolve null read/update anomalies.
7. Define hybrid delete behavior in repository/service: soft-delete marker/status now, hide deleted conversations from reads/messages, enqueue or schedule hard purge of conversation, messages, pending actions, and tool logs.
8. Remove duplicate/competing pending-action creation paths by consolidating ownership between chat service and interceptor logic across [internal/chat/service.go](internal/chat/service.go) and [pkg/grpc/interceptor.go](pkg/grpc/interceptor.go).
9. Remove agent chat compatibility route entirely (404 by absence) and migrate health endpoint to chat ownership; update startup wiring in [cmd/orbit-core/main.go](cmd/orbit-core/main.go) if needed.
10. Replace cleanup-driven action expiry with apply-time validity checks in [internal/chat/service.go](internal/chat/service.go): when confirm/apply is requested, re-validate status, TTL, idempotency key, and conflict constraints at execution time; mark action status as expired/invalid when needed but keep the full action record for audit/history.
11. Add regression tests for contracts and bugs: new tests in [internal/chat/service_test.go](internal/chat/service_test.go), [internal/chat/repository_test.go](internal/chat/repository_test.go), and routing assertions in [internal/gateway/service_test.go](internal/gateway/service_test.go); verify OpenAPI regeneration via [scripts/merge-openapi.sh](scripts/merge-openapi.sh).

**Verification**
- Run unit tests with go test ./... and ensure new chat service/repository tests cover:
- strict 404 for unknown conversation ID on POST message
- 5xx contract on provider failure
- message persistence independent of provider success path
- conversation create/delete lifecycle behavior (including soft-delete visibility)
- repository key mapping correctness for conversation_id, action_id, created_at, updated_at
- apply-time revalidation behavior on confirm/apply (expired, invalid, conflict, idempotency mismatch)
- action records are preserved for history/audit even after expiration or invalidation
- Validate API contract by curl against create, post message, get conversation, delete conversation, and health routes.
- Rebuild OpenAPI and run existing verification script to ensure spec consistency.

**Decisions**
- Single owner: Chat Service only.
- Message flow: conversation ID required and must exist.
- Provider failure: return 5xx structured error.
- Agent chat endpoint: remove now (no compatibility shim).
- Health ownership: move to chat route.
- Delete semantics: hybrid (soft delete now, hard purge later).
- Action expiry semantics: validate at confirm/apply time; do not rely on periodic cleanup for correctness.
- Audit retention: keep action records regardless of final status (confirmed/cancelled/expired/failed/invalid).

**High-impact improvements beyond your draft**
- Action ID format bug (likely production-breaking): pending actions are created as action_* but confirmation/cancel/get require UUID parse, so valid action IDs can be rejected as 400. See [internal/chat/repository.go](internal/chat/repository.go), [pkg/grpc/interceptor.go](pkg/grpc/interceptor.go), [internal/chat/service.go](internal/chat/service.go).
- Mongo DB config mismatch risk: chat repo is initialized with SQL DB name, not Mongo DB name; this can silently point chat collections at the wrong database. See [cmd/orbit-core/main.go](cmd/orbit-core/main.go) and config split in [pkg/config/config.go](pkg/config/config.go).
- Request hardening: chat handlers decode directly from body without max size and without strict field validation; add MaxBytesReader + DisallowUnknownFields. See [internal/chat/service.go](internal/chat/service.go).
- gRPC timeout boundaries: chat->agent and action execution calls rely on ambient context only; add explicit per-call deadlines/circuit handling for predictable failure modes. See [internal/chat/service.go](internal/chat/service.go).
