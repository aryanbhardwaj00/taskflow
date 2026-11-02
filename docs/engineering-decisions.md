# Engineering Decisions & Problem Log

This log tracks architectural decisions, real issues encountered, deliberately simulated problems for learning, and technical trade-offs made throughout the development of TaskFlow.

---

## Log Template

Every entry follows this standardized format:

```markdown
### [Type] Title

- **Category:** (Actually Encountered | Deliberately Simulated | Architectural Decision)
- **Problem / Context:** What was the problem or design requirement?
- **Root Cause / Why It Matters:** Why did this happen or why must we address it?
- **How We Identified It:** Observability, compiler error, panic, benchmark, test failure, etc.
- **Alternatives Considered:**
  1. Option A
  2. Option B
- **Selected Solution:** What did we implement?
- **Why We Selected It:** Justification based on requirements and Go idioms.
- **Trade-offs:** What are the downsides or compromises?
- **Verification / Testing:** How did we verify that this works as intended?
```

---

## ADR-001: Layered Clean Architecture & Standard Go Project Layout

- **Category:** Architectural Decision
- **Problem / Context:** Maintainable, testable, and idiomatic codebase structure for a task management REST API with PostgreSQL, Redis, and clean dependency injection.
- **Selected Solution:** Layered Clean Architecture within standard Go layout (`cmd/api`, `internal/...`).

---

## ADR-002: Graceful Shutdown and Custom http.Server Timeouts

- **Category:** Architectural Decision
- **Problem / Context:** Prevent dropped in-flight connections on container redeployment and protect against connection leaks (Slowloris).
- **Selected Solution:** Configured custom `&http.Server` with explicit read/write/idle timeouts and captured `SIGINT`/`SIGTERM` via `signal.Notify` for graceful 5s shutdown.

---

## ADR-003: 12-Factor App Environment Configuration & Fail-Fast Validation

- **Category:** Architectural Decision
- **Problem / Context:** Portable configuration across dev, CI, and containers without hardcoded secrets.
- **Selected Solution:** Implemented `internal/config/config.go` with grouped structs, `.env` loading, and startup fail-fast validation.

---

## ADR-004: PostgreSQL Connection Pool Tuning & Docker Volume Initialization Gotchas

- **Category:** Actually Encountered
- **Problem / Context:** Port conflict on `5432`, missing `taskflow_db` volume state, nil pointer panic on startup failure, and password mismatch after changing compose file.
- **Selected Solution:** Mapped host port `5431:5432`; used `log.Fatalf` in `main.go` on DB failure; added nil-check in `db.Close()`; used `docker compose down -v` to reset volume cleanly.

---

## ADR-005: Bcrypt Password Hashing with Work Factor Tuning & DTO Protection

- **Category:** Architectural Decision
- **Problem / Context:** User passwords must be stored securely to prevent credential recovery in case of data breaches.
- **Selected Solution:** Selected bcrypt with `DefaultCost` (10), enforced `json:"-"` struct tag on `model.User.PasswordHash`, and separated database models from API DTOs.

---

## ADR-006: JWT Signing (HS256) & Explicit Algorithm Verification

- **Category:** Architectural Decision
- **Problem / Context:** Stateless, horizontal-scaling authentication without querying the database on every HTTP request.
- **Selected Solution:** Implemented HS256 JWT access tokens with embedded `UserClaims` and enforced `token.Method.(*jwt.SigningMethodHMAC)` type assertion before evaluating the signature.

---

## ADR-007: Gin Middleware Interceptor Pattern & Context-Scoped Identity

- **Category:** Architectural Decision
- **Problem / Context:** Handlers require verified user identity without duplicating JWT parsing.
- **Selected Solution:** Created `AuthMiddleware` checking `Authorization: Bearer <token>`, validating claims, and binding `userID`, `userEmail`, and `userRole` into `gin.Context`.

---

## ADR-008: Object-Level Authorization (IDOR Defense) & Role-Based Access Control (RBAC)

- **Category:** Architectural Decision
- **Problem / Context:** Prevent users from tampering with other users' tasks via URL manipulation (IDOR). Enforce administrative privileges.
- **Selected Solution:** Enforced object ownership in SQL queries (`WHERE id = $1 AND user_id = $2`) with `tag.RowsAffected() == 0` checking. Implemented `RequireRoles` middleware for route-level RBAC returning `HTTP 403 Forbidden`.

---

## ADR-009: In-Memory Redis Client Initialization & Host Port Mapping

- **Category:** Actually Encountered & Architectural Decision
- **Problem / Context:** Host port `6379` already in use; need an in-memory caching client with connection pooling and startup connectivity check.
- **Selected Solution:** Mapped host port `6378:6379` in `docker-compose.yml`, matched `REDIS_PORT=6378` in `.env`, and initialized `internal/infra/redis.go` with `PoolSize: 20` and immediate `rdb.Ping(ctx)`.

---

## ADR-010: Cache-Aside Caching Strategy & Invalidation via Deletion

- **Category:** Architectural Decision
- **Problem / Context:** High database read load on frequent task lookups; risks of stale or inconsistent cache data when tasks are updated or deleted.
- **Selected Solution:** Implemented the Cache-Aside pattern in `internal/service/task_service.go`: Read from Redis $\rightarrow$ on miss query PostgreSQL and populate Redis with TTL. On update/delete, update PostgreSQL first and immediately delete (`DEL`) the Redis key. Gracefully degraded if Redis is unreachable.

---

## ADR-011: Distributed Redis Rate Limiting & Fail-Open Throttling

- **Category:** Architectural Decision
- **Problem / Context:** Protecting public endpoints (login/register) from brute-force password guessing and preventing API resource exhaustion across distributed server instances.
- **Selected Solution:** Implemented `RateLimiter` middleware in `internal/middleware/rate_limit_middleware.go` using Redis `INCR` and `EXPIRE` grouped by time buckets (`unixTime / windowSec`). Identifies clients by `userID` (authenticated) or `ClientIP` (unauthenticated). Enforces "Fail Open" (logs warning and permits traffic if Redis blips). Sets standard headers (`X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After`).

---

## ADR-012: Refresh Token Rotation (RTR), State Invalidation & Idempotent Logout

- **Category:** Actually Encountered & Architectural Decision
- **Problem / Context:**
  1. Stateless JWT access tokens cannot be revoked if stolen, but forcing users to re-login frequently hurts UX.
  2. Logout endpoint returned `200 OK` even when passed an already-deleted or tampered refresh token.
- **Root Cause / Why It Matters:**
  - Simple JWTs force an unacceptable compromise between security (short TTL) and user convenience (long TTL).
  - Redis `DEL key` returns `0` if the key does not exist, with `err == nil`.
- **How We Identified It:** Manual Postman testing of `POST /auth/logout` with reused and invalid tokens.
- **Alternatives Considered:**
  1. **Strict Session Verification on Logout:** Return `400 Bad Request` or `401 Unauthorized` if Redis returns 0 deleted keys.
     - *Cons:* Confusing user experience (user clicked logout and wants confirmation of being logged out; showing an error makes them think they are still logged in); leaks token validity information to attackers.
  2. **Idempotent Logout (Selected):** Always return `200 OK` regardless of whether the key existed.
- **Selected Solution:**
  - Dual-token architecture: Short-lived stateless access token (15m) + Long-lived stateful refresh token (7 days in Redis).
  - Implemented **Refresh Token Rotation (RTR)**: Every `/auth/refresh` call deletes the old token and issues a brand-new pair. Replay attempts indicate token theft and return `401`.
  - Kept `Logout` strictly **idempotent**: The target state ("session is invalid") is guaranteed.
- **Why We Selected It:** Provides resilient client retry semantics, avoids frustrating error prompts on user logout, and prevents token probing reconnaissance.
- **Verification / Testing:** Tested token refresh issuing new pairs, verified that presenting the old rotated token returns `401 Unauthorized`, and verified Redis key cleanup.
