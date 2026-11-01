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
- **Problem / Context:** We need a maintainable, testable, and idiomatic codebase structure for a task management REST API that supports PostgreSQL, Redis caching, JWT auth, and clean dependency injection.
- **Selected Solution:** Layered Clean Architecture within standard Go layout (`cmd/api`, `internal/...`).

---

## ADR-002: Graceful Shutdown and Custom http.Server Timeouts

- **Category:** Architectural Decision
- **Problem / Context:** Standard `router.Run(":8080")` abruptly kills in-flight connections on container redeployment and lacks network I/O timeouts, making the server vulnerable to resource exhaustion.
- **Selected Solution:** Configured custom `&http.Server` with `ReadTimeout: 10s`, `WriteTimeout: 10s`, `IdleTimeout: 60s`, executed `srv.ListenAndServe()` in a separate goroutine, and captured `SIGINT`/`SIGTERM` via `signal.Notify` to trigger `srv.Shutdown(ctx)` with a 5-second deadline.

---

## ADR-003: 12-Factor App Environment Configuration & Fail-Fast Validation

- **Category:** Architectural Decision
- **Problem / Context:** Applications need portable configuration across local development, CI pipelines, and production container environments without rebuilding code or checking secrets into version control.
- **Selected Solution:** Implemented `internal/config/config.go` with grouped structs (`App`, `DB`, `Redis`, `JWT`), `godotenv.Load()` with graceful fallback for production containers, `os.LookupEnv()` with defaults, and a `Validate()` method that prevents booting in production with weak secrets.

---

## ADR-004: PostgreSQL Connection Pool Tuning & Docker Volume Initialization Gotchas

- **Category:** Actually Encountered
- **Problem / Context:** Port conflict on `5432`, missing `taskflow_db` volume state, nil pointer panic on startup failure, and password mismatch after changing compose file.
- **Selected Solution:** Mapped host port `5431:5432`; used `log.Fatalf` in `main.go` on DB failure; added nil-check in `db.Close()`; used `docker compose down -v` to reset volume cleanly.

---

## ADR-005: Bcrypt Password Hashing with Work Factor Tuning & DTO Protection

- **Category:** Architectural Decision
- **Problem / Context:** User passwords must be stored securely to prevent credential recovery in the event of a database breach or unauthorized dump.
- **Selected Solution:** Selected bcrypt with `DefaultCost` (10). Enforced `json:"-"` struct tag on `model.User.PasswordHash`, normalized email inputs, and separated database models from API DTOs.

---

## ADR-006: JWT Signing (HS256) & Explicit Algorithm Verification

- **Category:** Architectural Decision
- **Problem / Context:** Need stateless, horizontal-scaling authentication for API endpoints without querying the database on every single incoming HTTP request.
- **Selected Solution:** Implemented HS256 JWT access tokens with embedded `UserClaims` (`UserID`, `Email`, `Role`, `ExpiresAt`). In `ValidateToken`, explicitly enforced `token.Method.(*jwt.SigningMethodHMAC)` type assertion before evaluating the signature.

---

## ADR-007: Gin Middleware Interceptor Pattern & Context-Scoped Identity

- **Category:** Architectural Decision
- **Problem / Context:** Handlers require verified user identity without duplicating JWT parsing or allowing unauthorized requests to invoke business handlers.
- **Selected Solution:** Created `AuthMiddleware` checking `Authorization: Bearer <token>`, validating claims, and binding `userID`, `userEmail`, and `userRole` into `gin.Context`. Implemented type-safe helper functions (`GetUserID`, `GetUserRole`) for handlers.

---

## ADR-008: Object-Level Authorization (IDOR Defense) & Role-Based Access Control (RBAC)

- **Category:** Architectural Decision
- **Problem / Context:** Attackers could manipulate task IDs to read, update, or delete other users' tasks (Insecure Direct Object Reference - IDOR). Administrative endpoints must be restricted to authorized roles.
- **Selected Solution:** Enforced object ownership at the SQL query level (`WHERE id = $1 AND user_id = $2`) and evaluated `tag.RowsAffected() == 0` to return `ErrTaskNotFound`. Implemented `RequireRoles` middleware for route-level RBAC returning `HTTP 403 Forbidden`.

---

## ADR-009: In-Memory Redis Client Initialization & Host Port Mapping

- **Category:** Actually Encountered & Architectural Decision
- **Problem / Context:** 
  1. Default Redis port `6379` was already bound on the host machine.
  2. Need an in-memory caching and transient state client with bounded connection pooling and fail-fast startup connectivity check.
- **Root Cause / Why It Matters:**
  - Running a container on a port already bound by a local service causes Docker startup network collision.
  - Creating a Redis client lazily without pinging can lead to runtime crashes under traffic if credentials or network reachability fail.
- **Selected Solution:** 
  - Mapped host port `6378:6379` in `docker-compose.yml` and matched `REDIS_PORT=6378` in `.env`.
  - Implemented `internal/infra/redis.go` using `go-redis/v9` with `PoolSize: 20`, `MinIdleConns: 5`, and an immediate `rdb.Ping(ctx)` on startup.
- **Why We Selected It:** Prevents local development port collisions and guarantees that Redis connectivity is verified before accepting HTTP traffic.
- **Verification / Testing:** Tested `GET /api/v1/redis-test`, verified key storage and 60-second TTL via `redis-cli ttl test:greeting`.
