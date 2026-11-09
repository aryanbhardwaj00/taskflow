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
- **Problem / Context:** Stateless JWT access tokens cannot be revoked if stolen, but forcing users to re-login frequently hurts UX. Logout endpoint returned `200 OK` even when passed an already-deleted or tampered refresh token.
- **Selected Solution:** Dual-token architecture (15m stateless JWT + 7d stateful Redis refresh token) with Refresh Token Rotation (RTR) on `/auth/refresh`. Kept `Logout` strictly idempotent returning `200 OK` to ensure resilient client retry semantics and prevent token probing attacks.

---

## ADR-013: Table-Driven Testing, Interface Mocking & In-Memory HTTP Dispatching

- **Category:** Architectural Decision
- **Problem / Context:** Need automated test coverage for domain services and HTTP middleware without creating slow, brittle dependencies on running PostgreSQL/Redis instances during unit testing.
- **Root Cause / Why It Matters:**
  - Tests that require live databases are slow, prone to data pollution, and cannot run in isolated PR review environments easily.
  - Writing separate test functions for every edge case leads to code duplication.
- **How We Identified It:** Go standard testing idioms and CI/CD fast feedback requirements.
- **Alternatives Considered:**
  1. Spinning up Docker containers for all unit tests (too slow for rapid TDD).
  2. Manual mock structs without libraries.
  3. `testify/mock` with table-driven tests and `net/http/httptest` for handlers/middlewares.
- **Selected Solution:**
  - Implemented Table-Driven Tests with `t.Run(tc.name, ...)` for all scenarios (success, edge cases, failures).
  - Used `testify/mock` to mock `UserRepository` and `AuthService` interfaces with exact argument expectations (`mock.MatchedBy`).
  - Used `net/http/httptest.NewRecorder()` and `router.ServeHTTP()` for in-memory HTTP middleware verification.
- **Why We Selected It:** Tests execute in milliseconds ($<10\text{ms}$), require zero external infrastructure, provide 100% deterministic assertion of error handling branches, and decouple testing from network layers.
- **Verification / Testing:** Executed `go test -v ./...` verifying that all service and middleware scenarios pass in $<0.75$ seconds.

---

## ADR-014: Integration Testing with Live PostgreSQL, Build Tags & Table Truncate Isolation

- **Category:** Architectural Decision
- **Problem / Context:** Unit tests verify isolated Go logic, but cannot verify SQL syntax errors, database-level schema constraints (foreign keys, unique indexes), or driver deserialization (`UUID`, `TIMESTAMPTZ`).
- **Root Cause / Why It Matters:**
  - Mocking the database can hide bugs: invalid SQL column names, failing foreign key references, or unhandled PostgreSQL error codes.
  - Running database tests during routine `go test ./...` in development or lightweight CI environments causes failures if a live database container is not running.
- **Alternatives Considered:**
  1. Transaction Rollback (`tx.Rollback()` per test): Fast, but does not test real committed data or repository functions that internally use connection pools.
  2. Mocking SQL driver (`sqlmock`): Still mocks SQL and does not test the real PostgreSQL engine or Postgres-specific extensions.
  3. Real PostgreSQL container with Build Tags (`//go:build integration`) and `TRUNCATE TABLE ... CASCADE`: Provides 100% realistic database verification while keeping unit tests fast.
- **Selected Solution:**
  - Placed `//go:build integration` on all database integration test files.
  - Used a shared `setupTestDB(t)` helper connecting to Dockerized PostgreSQL on port `5431`.
  - Implemented cleanup teardown using `TRUNCATE TABLE tasks, users CASCADE;` inside `defer teardown()`.
  - Created end-to-end repository tests verifying:
    1. Schema generation of UUIDs and timestamps upon `INSERT`.
    2. PostgreSQL error code `23505` correctly mapped to `ErrDuplicateEmail`.
    3. PostgreSQL foreign key constraint violation when inserting orphaned tasks.
    4. Object-level authorization (IDOR) verification in SQL (`WHERE id = $1 AND user_id = $2`) preventing cross-user reads, updates, and deletes.
    5. Pagination and status filtering.
- **Why We Selected It:** Guarantees that our actual SQL queries, constraints, and connection pooling work seamlessly with real PostgreSQL, while `-tags=integration` ensures standard unit tests remain instantaneous.
- **Verification / Testing:** Ran `go test -tags=integration -v ./internal/repository/...` executing 7 comprehensive test suites in 0.528s with zero data leakage between tests.

---

## ADR-015: Multi-Stage Docker Build, Alpine Stripping & Docker Compose Orchestration

- **Category:** Architectural Decision & Deliberately Simulated
- **Problem / Context:** Production deployment requires a secure, minimal container image containing only runtime dependencies, along with unified local orchestration where PostgreSQL, Redis, database migrations, and the Go API run in an isolated Docker network.
- **Root Cause / Why It Matters:**
  - A standard Go build image (`golang:1.26-alpine`) contains compilers, package managers, and caches weighing $\sim 1\text{ GB}$, increasing attack surface and CI/CD push/pull latency.
  - Running containers as `root` creates high vulnerability risks in case of remote code execution (container escape).
  - Starting the API before PostgreSQL and Redis finish initializing results in startup crash-loops.
- **Alternatives Considered:**
  1. Single-stage container image: Simple, but huge image size ($>800\text{MB}$) and leaves development tools in production.
  2. `scratch` base image: Extremely small, but lacks CA certificates (breaks external HTTPS requests) and has no shell for health check diagnostics.
  3. Two-stage Alpine build (`golang:1.26-alpine` builder $\rightarrow$ `alpine:3.20` runner): Minimal footprint ($\sim 75\text{MB}$ total disk usage with certificates and non-root user).
- **Selected Solution:**
  - **Stage 1 (Builder):** Built statically linked binary using `CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w"`.
  - **Stage 2 (Runner):** Alpine 3.20 base image, installed `ca-certificates` and `tzdata`, created unprivileged non-root user (`appuser:10001`), copied only the binary, and added a native `HEALTHCHECK`.
  - **Docker Compose:** Added `api` service with `depends_on` conditions (`service_healthy` for postgres/redis and `service_completed_successfully` for migrations).
- **Why We Selected It:** Slashes image size by $>90\%$, guarantees zero dynamic C library runtime dependencies, secures runtime permissions, and ensures zero startup race conditions between database migrations and API booting.
- **Verification / Testing:** Built `taskflow-api:latest` successfully via `docker build -t taskflow-api:latest .` resulting in a compact 75.5MB image.

---

## ADR-016: GitHub Actions Automated CI/CD Pipeline & Pull Request Gates

- **Category:** Architectural Decision
- **Problem / Context:** Relying on developers to run unit tests and Docker builds manually on local machines before merging to `main` is error-prone and leads to broken deployments.
- **Root Cause / Why It Matters:**
  - Code that passes locally might fail on clean machines or in containers due to uncommitted files, differing local OS libraries, or cache pollution.
  - Pull requests need automated regression verification before human code reviews.
- **Alternatives Considered:**
  1. Manual pre-commit verification (brittle, easily skipped).
  2. Local git pre-push hooks (runs only on developer's machine, does not run on PRs).
  3. GitHub Actions cloud runner pipeline triggered on `push` and `pull_request` targeting `main`.
- **Selected Solution:**
  - Configured `.github/workflows/ci.yml`.
  - Step 1: `actions/setup-go@v5` caching dependencies and running `go vet` static analysis.
  - Step 2: Running service and middleware unit tests.
  - Step 3: Setting up Docker Buildx and executing a test build of the multi-stage Docker container.
- **Why We Selected It:** Zero server setup, integrates natively with GitHub Pull Request UI, enforces that no code reaches `main` without passing all tests, and acts as the gatekeeper for CD deployments.
- **Verification / Testing:** Pushed feature branch `feature-Redis`, triggered GitHub Actions in the cloud, observed both test and docker build jobs pass with green badges, and successfully executed a clean PR merge into `main`.

---

## ADR-017: AWS EC2 Cloud Deployment, Cross-Compilation Artifacts & Teardown Lifecycle

- **Category:** Actually Encountered & Architectural Decision
- **Problem / Context:** Deploying TaskFlow to AWS EC2 (`t3.micro`) exposed severe build slowdowns when compiling dependencies inside Docker on a single-core, 1GB RAM burstable VM (swap thrashing and CPU credit throttling).
- **Root Cause / Why It Matters:**
  - `t3.micro` instances have only 1 vCPU and 1 GB RAM. Building complex Go codebases inside Docker triggers memory paging to EBS swap files and exhausts burst CPU credits.
  - Compiling directly on production servers violates the build/release/run separation principle (12-Factor App, Factor V).
- **Alternatives Considered:**
  1. Compiling on the server inside Docker: Too slow (8–10 minutes) and risks OOM termination on micro instances.
  2. Upgrading to high-spec EC2 instance (`t3.large`): Incurs substantial cloud costs.
  3. Go Cross-Compilation on developer workstation (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`) + SCP artifact delivery + minimal Alpine runner container: Builds in 3 seconds, deploys in 2 seconds.
- **Selected Solution:**
  - Cross-compiled static binary on host using `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w"`.
  - Transferred binary via SSH `scp` to EC2 instance.
  - Used minimal Alpine runner `Dockerfile` to package and run the pre-built binary.
  - Orchestrated full stack (Postgres + Redis + API + Migrations) using Docker Compose with internal DNS resolution.
  - Opened Security Group ingress ports `22` (SSH) and `8080` (HTTP).
  - Decommissioned instance via `aws ec2 terminate-instances` to guarantee $0 lingering costs.
- **Why We Selected It:** Eliminates build bottlenecks on low-spec cloud VMs, leverages Go's zero-dependency static compilation, and proves cloud orchestration end-to-end.
- **Verification / Testing:** Hit live endpoint `http://16.178.30.160:8080/health` from external internet returning HTTP 200 OK, registered a live user with HTTP 201 Created, and verified clean instance termination.

---

## ADR-018: Full-Stack Test Automation, Miniredis Simulation & Codebase Sanitization

- **Category:** Architectural Decision
- **Problem / Context:** While data repositories had live PostgreSQL integration tests and the authentication service had unit tests, `TaskService`, `RateLimiter` middleware, and HTTP handlers lacked automated unit test coverage. Unused placeholder files (`logger_middleware.go`) and planning phase comments also lingered in the codebase.
- **Root Cause / Why It Matters:**
  - Missing tests on business and HTTP layers risk regressions when refactoring task business rules, pagination limits, or input validation.
  - Testing Redis-backed rate limiting in CI without a live Redis server requires either brittle mocking of Redis internals or an accurate in-memory implementation.
  - Leftover placeholder files and comments signal unfinished or prototype-level code quality.
- **Alternatives Considered:**
  1. Spinning up Dockerized Redis in CI for middleware tests (adds 20-30s CI startup overhead).
  2. Manual mock structs for Redis commands (brittle and complex to maintain for `INCR`/`EXPIRE`/`TTL`).
  3. `alicebob/miniredis/v2` in-memory engine combined with `testify/mock` and `httptest.NewRecorder`.
- **Selected Solution:**
  - Removed empty `internal/middleware/logger_middleware.go` and sanitized legacy phase comments across `health_handler.go` and `postgres.go`.
  - Added unit test suites for `TaskService` (`internal/service/task_service_test.go`) covering `CreateTask`, `GetTask`, `ListTasks`, `UpdateTaskStatus`, and `DeleteTask` with `MockTaskRepository`.
  - Added unit test suites for `RateLimiter` (`internal/middleware/rate_limit_middleware_test.go`) utilizing `alicebob/miniredis/v2` to verify quota enforcement, HTTP 429 status, RFC rate limit headers, and fail-open behavior.
  - Added HTTP handler tests (`internal/handler/auth_handler_test.go`, `task_handler_test.go`, `health_handler_test.go`) with `httptest.NewRecorder` verifying status codes, payload binding, and route guards.
  - Updated GitHub Actions workflow (`.github/workflows/ci.yml`) to run `./internal/handler/...` tests on every pull request and push to `main`.
- **Why We Selected It:** Delivers complete test coverage across service, middleware, and handler layers in $<1.5$ seconds with zero external runtime dependencies.
- **Verification / Testing:** Executed `go test -v ./internal/service/... ./internal/middleware/... ./internal/handler/...` passing all 24 unit test cases cleanly.

---

## ADR-019: Prometheus Metrics Instrumentation, High-Cardinality Defense & Grafana Provisioning

- **Category:** Architectural Decision
- **Problem / Context:** Raw text application logs are insufficient for monitoring real-time system health, p95/p99 latency distributions, and throughput at scale without overwhelming storage and query performance.
- **Root Cause / Why It Matters:**
  - High-traffic microservices generate gigabytes of log output, making aggregate rate calculation slow and costly.
  - Recording metric labels with dynamic URL parameters (e.g., `/api/v1/tasks/123e4567...`) creates a "cardinality explosion" that can exhaust Prometheus server memory and degrade TSDB query speeds.
  - Requiring manual UI setup for data sources in Grafana leads to configuration drift across environments.
- **Alternatives Considered:**
  1. Relying strictly on structured JSON logging.
  2. Push-based APM agent (e.g., Datadog/NewRelic) requiring proprietary vendor libraries and licensing costs.
  3. Pull-based CNCF standard: Prometheus client library (`client_golang`) + Grafana datasource provisioning.
- **Selected Solution:**
  - Implemented `PrometheusMetrics` middleware in `internal/middleware/metrics_middleware.go` tracking `http_requests_total` (Counter), `http_request_duration_seconds` (Histogram across 11 buckets), and `http_requests_in_flight` (Gauge).
  - Used Gin's `c.FullPath()` to record parameterized route patterns (`/api/v1/tasks/:id`) instead of raw request URLs, preventing cardinality explosion.
  - Exposed standard pull endpoint `GET /metrics` via `promhttp.Handler()` including Go runtime stats (heap allocations, goroutines, GC cycles).
  - Automated Grafana provisioning via `grafana/provisioning/datasources/datasource.yml` connecting to Prometheus (`http://prometheus:9090`) out of the box.
- **Why We Selected It:** Provides sub-millisecond in-memory metrics recording, standards-compliant PromQL querying, zero vendor lock-in, and automated dashboard visualization.
- **Verification / Testing:** Verified `/metrics` endpoint exports valid Prometheus text format; confirmed scrape target is UP in Prometheus dashboard; imported Go Runtime Dashboard (`14061`) in Grafana showing real-time traffic, goroutines, and latency.

---

## ADR-020: Kubernetes Declarative Orchestration, Self-Healing ReplicaSets & Zero-Downtime Health Probes

- **Category:** Architectural Decision & Deliberately Simulated
- **Problem / Context:** Docker Compose is constrained to single-node hosting. Production systems require multi-node cluster management, automated self-healing against process crashes, zero-downtime rolling updates, and elastic horizontal scaling.
- **Root Cause / Why It Matters:**
  - A single-node container failure takes down the service until manual intervention occurs.
  - Replacing a running container in Docker Compose produces brief downtime (502/503 errors).
  - Traffic spikes require manual configuration changes rather than declarative horizontal pod scaling.
- **Alternatives Considered:**
  1. Docker Swarm: Simpler, but lacks broad cloud-native ecosystem adoption and advanced autoscaling capabilities.
  2. AWS ECS: AWS-specific, creating cloud vendor lock-in.
  3. Kubernetes (K8s): CNCF industry standard with declarative state reconciliation, self-healing, and portable across all cloud providers.
- **Selected Solution:**
  - Authored Kubernetes manifests in `k8s/`:
    - `k8s/deployment.yaml`: Replicated Pod management (`replicas: 2`) with explicit CPU/memory requests (`50m/64Mi`) and limits (`200m/128Mi`).
    - `k8s/service.yaml`: `NodePort` service abstraction decoupling internal routing from dynamic pod IP changes.
    - `k8s/postgres.yaml` & `k8s/redis.yaml`: In-cluster PostgreSQL and Redis deployments + `ClusterIP` services for localized DNS resolution.
  - Configured `readinessProbe` and `livenessProbe` hitting `/health` on port 8080 to guarantee traffic only routes to fully initialized containers and deadlocked instances are restarted automatically.
- **Why We Selected It:** Guarantees self-healing within seconds, enables zero-downtime rolling deployments, and provides seamless horizontal scaling via `kubectl scale`.
- **Verification / Testing:**
  1. Deployed full stack to local Minikube cluster (`kubectl apply -f k8s/`).
  2. Intentionally killed a running pod (`kubectl delete pod ...`); observed Kubernetes autonomously detect drift and spawn an identical replacement in $<4$ seconds.
  3. Scaled from 2 to 5 replicas in 2 seconds (`kubectl scale deployment taskflow-api --replicas=5`) and back down cleanly.
  4. Verified connectivity through `kubectl port-forward svc/taskflow-api 8080:8080` handling active requests.
