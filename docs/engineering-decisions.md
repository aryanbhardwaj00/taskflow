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
- **Root Cause / Why It Matters:** 
  Without a strict separation of concerns:
  - Database queries get mingled with HTTP request decoding, making unit testing impossible without spinning up a real DB.
  - Business logic depends directly on HTTP frameworks (Gin), locking the domain to HTTP transports.
  - Circular dependencies easily occur in Go if packages are poorly separated.
- **How We Identified It:** Standard backend best practice in production Go systems.
- **Alternatives Considered:**
  1. **Flat package structure:** Everything in `package main`.
     - *Pros:* Simple for tiny micro-scripts.
     - *Cons:* Becomes unmaintainable after 3-4 files; global variables and tight coupling.
  2. **Package by feature (e.g., `pkg/users`, `pkg/tasks`):** Each package contains its own handler, service, repo.
     - *Pros:* Colocates all logic for a single business capability.
     - *Cons:* Can lead to cyclic import dependencies in Go when entities reference each other (e.g. Tasks referencing Users).
  3. **Layered Clean Architecture (`internal/handler`, `internal/service`, `internal/repository`):**
     - *Pros:* Clear unidirectional dependency flow (`Handler -> Service -> Repository`), zero circular imports, easy interface mocking with Go interfaces.
     - *Cons:* Slightly more boilerplate (DTOs, interface definitions).
- **Selected Solution:** Layered Clean Architecture within standard Go layout (`cmd/api`, `internal/...`).
- **Why We Selected It:** Unidirectional dependency graph maps naturally to Go. Go interfaces allow the service layer to define repository interfaces that it needs, enabling clean mock-based unit tests.
- **Trade-offs:** Requires constructor-based dependency injection wiring in `main.go` and boundary data mapping (DTO to Domain Model).
- **Verification / Testing:** Verified that `internal/` packages enforce package encapsulation and packages compile without circular dependencies.

---

## ADR-002: Graceful Shutdown and Custom http.Server Timeouts

- **Category:** Architectural Decision
- **Problem / Context:** Standard `router.Run(":8080")` abruptly kills in-flight connections on container redeployment and lacks network I/O timeouts, making the server vulnerable to resource exhaustion.
- **Root Cause / Why It Matters:**
  1. **Slowloris Attacks / Leaked Connections:** Default `http.Server` has 0 timeouts (`ReadTimeout` and `WriteTimeout`). Clients that open connections and trickle data slowly keep server goroutines and file descriptors open indefinitely.
  2. **Abrupt Process Termination:** When Docker or Kubernetes restarts or terminates a container, it sends `SIGTERM`. If the process immediately terminates, active database operations and HTTP responses are aborted, resulting in HTTP 502/504 errors for end users.
- **How We Identified It:** Standard production microservice design pattern.
- **Alternatives Considered:**
  1. **`router.Run(":8080")`:** Simplest Gin shortcut, but ignores OS signals and sets no HTTP timeouts.
  2. **Standard `http.Server` with signal trapping and `srv.Shutdown(ctx)`:**
- **Selected Solution:** Configured custom `&http.Server` with `ReadTimeout: 10s`, `WriteTimeout: 10s`, `IdleTimeout: 60s`, executed `srv.ListenAndServe()` in a separate goroutine, and captured `SIGINT`/`SIGTERM` via `signal.Notify` to trigger `srv.Shutdown(ctx)` with a 5-second deadline.
- **Why We Selected It:** Guarantees zero dropped in-flight requests during rolling deployments and protects the service against idle or slow-read connection leaks.
- **Trade-offs:** Slightly more boilerplate code in `main.go`.
- **Verification / Testing:** Tested running the server binary, issuing `curl` requests, and sending `SIGINT` (Ctrl+C). Verified terminal logs confirm `"Initiating graceful shutdown..."` and `"Server exiting successfully."` without errors.

---

## ADR-003: 12-Factor App Environment Configuration & Fail-Fast Validation

- **Category:** Architectural Decision
- **Problem / Context:** Applications need portable configuration across local development, CI pipelines, and production container environments without rebuilding code or checking secrets into version control.
- **Root Cause / Why It Matters:**
  - Hardcoded configuration requires recompilation per environment.
  - Storing `.env` in Git risks critical credential leaks.
  - Silent configuration defaults for security keys (like JWT secret) can lead to insecure production deployments.
- **How We Identified It:** The Twelve-Factor App methodology (Factor III: Config).
- **Alternatives Considered:**
  1. **JSON/YAML config files (`config.yaml`):** Flexible, but requires mounting files into containers and risk accidental commit of secret files.
  2. **Direct `os.Getenv()` calls scattered across packages:** Difficult to audit, no validation, prone to typos.
  3. **Centralized strongly typed `Config` struct with optional `.env` loading and startup validation:**
- **Selected Solution:** Implemented `internal/config/config.go` with grouped structs (`App`, `DB`, `Redis`, `JWT`), `godotenv.Load()` with graceful fallback for production containers, `os.LookupEnv()` with defaults, and a `Validate()` method that prevents booting in production with weak secrets.
- **Why We Selected It:** Guarantees centralized type safety, fails fast at startup if critical keys are missing/weak, and simplifies dependency injection.
- **Trade-offs:** Must maintain `.env.example` as new services are introduced.
- **Verification / Testing:** Verified port overrides via `.env` and environment variables (`HTTP_PORT=9090 ./bin/api`). Verified that missing `.env` does not crash the app.

---

## ADR-004: PostgreSQL Connection Pool Tuning & Docker Volume Initialization Gotchas

- **Category:** Actually Encountered
- **Problem / Context:**
  1. Port conflict on host port `5432` during Docker startup.
  2. Database `taskflow_db` reported missing (`SQLSTATE 3D000`) despite being declared in `docker-compose.yml`.
  3. Nil pointer dereference panic in `defer db.Close()` when connection failed.
  4. Authentication error (`SQLSTATE 28P01`) after updating `POSTGRES_PASSWORD` in `docker-compose.yml`.
- **Root Cause / Why It Matters:**
  - **Docker Volume Lifecycle:** PostgreSQL only runs cluster initialization scripts (`initdb`, creating `POSTGRES_DB` and setting `POSTGRES_PASSWORD`) when the data volume is brand new. Subsequent edits to environment variables in `docker-compose.yml` are ignored by an existing volume.
  - **Defensive Error Handling:** If initialization functions like `NewPostgresDB` return `(nil, err)` and callers proceed without terminating (`log.Fatalf`), deferred cleanup calls (`defer db.Close()`) dereference a `nil` pointer.
- **How We Identified It:** Compiler/runtime error output in WSL terminal (`SQLSTATE 28P01`, `SQLSTATE 3D000`, SIGSEGV panic).
- **Alternatives Considered:**
  - Ignoring errors and continuing with a disconnected state (antipattern).
  - Altering user credentials via `psql` vs resetting the volume via `docker compose down -v`.
- **Selected Solution:**
  1. Mapped host port `5431:5432` to avoid host conflicts and matched `DB_PORT=5431` in `.env`.
  2. Used `log.Fatalf` in `main.go` on DB failure to enforce fail-fast semantics.
  3. Added `if db != nil && db.Pool != nil` check inside `db.Close()`.
  4. Used `docker compose down -v` to cleanly re-initialize the volume with matching password and database.
- **Why We Selected It:** Ensures robust startup guarantees and prevents silent failures.
- **Verification / Testing:** Successfully connected to `localhost:5431/taskflow_db` with `pgx/v5` connection pool.

---

## ADR-005: Bcrypt Password Hashing with Work Factor Tuning & DTO Protection

- **Category:** Architectural Decision
- **Problem / Context:** User passwords must be stored securely to prevent credential recovery in the event of a database breach or unauthorized dump.
- **Root Cause / Why It Matters:**
  - Fast cryptographic algorithms (SHA-256, SHA-512, MD5) can be brute-forced at rates of billions of attempts per second using consumer GPUs.
  - Rainbow tables can instantly reverse unsalted hashes.
  - Exposing raw models to API responses risks leaking password hashes to the client.
- **How We Identified It:** OWASP Authentication & Password Storage Guidelines.
- **Alternatives Considered:**
  1. **SHA-256 with manual salt:** Fast, vulnerable to high-throughput GPU brute-forcing.
  2. **PBKDF2:** Strong, but less memory-hard than modern algorithms.
  3. **Argon2id:** State of the art, but requires more complex memory/thread tuning.
  4. **Bcrypt (`golang.org/x/crypto/bcrypt`):** Industry standard, adaptive CPU-hard algorithm with built-in salting and configurable work factor.
- **Selected Solution:** Selected bcrypt with `DefaultCost` (10, meaning $2^{10} = 1024$ key expansion rounds). Enforced `json:"-"` struct tag on `model.User.PasswordHash`, normalized email inputs (`strings.ToLower(strings.TrimSpace(email))`), and separated database models from API DTOs.
- **Why We Selected It:** Built-in cryptographic salt generation prevents rainbow table lookups; tunable cost factor keeps brute-forcing computationally infeasible; explicit `json:"-"` tags protect against accidental serialization leaks.
- **Trade-offs:** Bcrypt hashes max out at 72 bytes of password input and consume 50-100ms of CPU per hash (which is desirable for defense, but requires monitoring during high-volume registration spikes).
- **Verification / Testing:** Verified in `auth_service.go` that passwords under 8 characters are rejected and valid passwords produce standard 60-character bcrypt hash strings.
