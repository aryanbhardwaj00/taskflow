# Golang Backend Interview Preparation Notes

This document contains curated interview questions, practical design questions, scenario-based challenges, common mistakes, and model answers covering each phase of building TaskFlow.

---

## Phase 0: Go Project Layout & Package Architecture

#### Q1: Why do Go projects use the `internal/` directory? What makes it special?
- **Answer:** The `internal/` package path is a compiler-enforced access rule introduced in Go 1.4. Code located inside `internal/` can only be imported by packages within the parent tree that roots above `internal`. External packages outside the module cannot import anything from your `internal/` folder.

#### Q2: What is the difference between `cmd/` and `internal/`?
- **Answer:** `cmd/` contains application entry points (`main.go`). Files in `cmd/` are thin wrappers that wire configuration, infrastructure, and handlers. `internal/` holds the actual domain, services, and repositories.

---

## Phase 1: HTTP Server, Gin Routing & Graceful Shutdown

#### Q1: What is Graceful Shutdown, and why is it essential in containerized/cloud environments?
- **Answer:** Graceful shutdown stops accepting new incoming connections while allowing active in-flight HTTP requests to complete within a timeout context. In Kubernetes or AWS ECS, rolling deployments terminate containers; without graceful shutdown, clients experience dropped connections and HTTP 502 errors.

---

## Phase 2: Configuration Management & 12-Factor App

#### Q1: What is the 12-Factor App rule regarding configuration?
- **Answer:** Factor III specifies *"Store config in the environment"*. Configuration varies substantially across deploys, but code does not. Configuration must be strictly separated from code, injected via environment variables rather than baked into source files.

---

## Phase 3: PostgreSQL Connection Pooling & Migrations

#### Q1: Why use `pgx` over the standard `lib/pq` driver?
- **Answer:** `pgx` (v5) communicates using PostgreSQL's binary protocol, natively supports connection pooling via `pgxpool`, supports PostgreSQL-specific data types (UUID, JSONB, arrays, inet), and provides fine-grained query tracing.

---

## Phase 4: Models, Repository Pattern & Compile-Time Dependency Injection

#### Q1: What does "Accept interfaces, return structs" mean in Go?
- **Answer:** Functions/methods declare interfaces as parameters (`NewAuthService(repo UserRepository)`), keeping consumers decoupled and mockable. Constructors return concrete structs (`*PostgresUserRepository`), giving callers full visibility and freedom to bind to any matching interface.

---

## Phase 5: Password Security & Bcrypt Deep Dive

#### Q1: Why can't we use SHA-256 for password storage? Why Bcrypt?
- **Answer:** SHA-256 is designed to be extremely fast (billions of hashes/sec on GPUs). An attacker with leaked hashes can crack passwords in minutes. Bcrypt is a CPU/memory-hard key derivation function based on Blowfish. It is intentionally slow (~50-100ms per hash at default cost 10) and features automatic per-user salt generation, preventing GPU brute-forcing and rainbow tables.

---

## Phase 6: JWT Access Tokens, Symmetric Signing & Security Gotchas

#### Q1: Explain the three parts of a JSON Web Token.
- **Answer:** Header (algorithm/type), Payload (claims/data), and Signature ($\text{HMACSHA256}(H + "." + P, \text{secret})$). Base64URL-encoded, tamper-proof, but NOT encrypted.

---

## Phase 7: Gin Middleware Mechanics & Context Propagation

#### Q1: What is the difference between `c.Next()` and `c.Abort()`?
- **Answer:** `c.Next()` runs downstream handlers in the chain and returns control afterward (onion model). `c.Abort()` prevents subsequent handlers from executing. Calling `c.Abort()` does **not** stop the current function; an explicit `return` must follow.

---

## Phase 8: Object-Level Authorization (IDOR) & RBAC

#### Q1: What is an IDOR vulnerability and how do you prevent it in SQL?
- **Answer:** Attackers guessing IDs to tamper with other users' resources. We prevent it by enforcing ownership in the SQL query: `WHERE id = $1 AND user_id = $2`.

---

## Phase 9: Redis Architecture & In-Memory Operations

#### Q1: What is Redis and how does its architecture differ from PostgreSQL?
- **Answer:** Redis is an in-memory (RAM) key-value store ($<1\text{ms}$ latency); PostgreSQL is a disk/SSD-backed relational database (5-50ms latency). Redis handles volatile caches and transient counters; PostgreSQL is the durable single source of truth.

---

## Phase 10: Caching Strategies & The Cache-Aside Pattern

#### Q1: Why should you DELETE the cache instead of updating it when data changes?
- **Answer:** To prevent concurrency race conditions where an older write overrides a newer write in Redis due to network packet reordering. Deleting the cache key (`DEL key`) ensures the next read queries the freshest authoritative record from PostgreSQL.

---

## Phase 11: Distributed Rate Limiting & Denial of Service Defense

#### Q1: Why must rate limiting be distributed using Redis in containerized backends?
- **Answer:** In modern microservice architectures with multiple container instances behind a load balancer, in-memory rate limiters allow attackers to multiply their request budget. Redis provides a centralized, thread-safe, atomic counter across all instances.

---

## Phase 12: Dual-Token Architecture, Refresh Token Rotation & Session Revocation

### 1. Core Authentication Architecture Questions

#### Q1: Why is a single long-lived JWT a security disaster in production?
- **Answer:** JWTs are stateless. The server verifies them exclusively via cryptographic signatures without consulting a database. If an attacker intercepts a 30-day JWT, they have 30 days of unrestricted access, and the server has **no mechanism to revoke or invalidate** the token.

#### Q2: How does the Dual-Token (Access + Refresh) pattern solve this?
- **Answer:**
  - **Access Token:** Very short-lived (15 minutes). Stateless. Sent with every API call. If intercepted, the exposure window is minimal.
  - **Refresh Token:** Long-lived (7 days). Stateful. Stored in Redis. Sent *only* to `/auth/refresh` when the access token expires. Can be revoked immediately by deleting the Redis key (`DEL refreshtoken:<token>`).

#### Q3: What is Refresh Token Rotation (RTR) and what attack does it detect?
- **Answer:**
  - **How It Works:** Every time `/auth/refresh` is called, the server invalidates the used refresh token and issues a brand-new access token AND a brand-new refresh token. Each refresh token is strictly single-use.
  - **Attack Detected:** **Replay Attacks / Token Theft**. If an attacker steals a refresh token and uses it, the legitimate user will eventually attempt to use the same token. When the server detects an attempt to use an already-invalidated token, it flags a security breach and revokes all active sessions for that user.

#### Q4: Why is Logout implemented as an Idempotent operation?
- **Answer:**
  - An operation is idempotent if executing it multiple times produces the exact same end state ($f(f(x)) = f(x)$).
  - When a user logs out, their goal is to ensure their session is terminated. If a network retry or accidental double-click sends two logout requests, returning `400` or `401` on the second request alarms the user into thinking logout failed.
  - Furthermore, returning `200 OK` regardless of whether the token was already deleted prevents attackers from using the logout endpoint to test if a given token string is active.

---

## Phase 13: Unit Testing, Table-Driven Tests & Interface Mocking

#### Q1: What is the core difference between a Unit Test and an Integration Test?
- **Answer:**
  - **Unit Test:** Tests an isolated piece of application code (e.g., a function, method, or service) in memory. External dependencies (databases, Redis, network calls) are **mocked or stubbed**. Unit tests run in milliseconds ($<10\text{ms}$) and verify pure business logic branching.
  - **Integration Test:** Tests how multiple system components collaborate with real infrastructure (e.g., real PostgreSQL, real Redis, HTTP network stack). It verifies that database queries have valid SQL syntax, schema constraints (unique indexes, foreign keys) actually fire, and wire serializations succeed.

#### Q2: Why are Table-Driven Tests considered the gold standard in Go?
- **Answer:**
  - Go does not have built-in parameterized testing annotations like `@ParameterizedTest` in JUnit.
  - Table-driven tests use an anonymous slice of structs containing test cases (`name`, inputs, mocks, expected outputs).
  - Inside a loop, each case is executed with `t.Run(tc.name, func(t *testing.T) { ... })`.
  - **Benefits:** Clean separation of test data from execution logic, zero code duplication when adding new edge cases, and independent subtest failure reporting with `t.Parallel()` capability.

#### Q3: When should you use `httptest.ResponseRecorder` instead of starting a live HTTP server?
- **Answer:**
  - `net/http/httptest.NewRecorder()` implements the `http.ResponseWriter` interface entirely in memory.
  - Instead of binding a TCP port and making network system calls (`http.Get("http://localhost:8080/...")`), `router.ServeHTTP(recorder, req)` dispatches the HTTP request directly through the handler chain synchronously in memory.
  - It runs $100\times$ faster, eliminates TCP port collision risks in CI pipelines, and allows inspecting headers, status codes, and JSON bodies directly.

---

## Phase 14: Integration Testing, Build Tags & Database Test Isolation

#### Q1: Why do we use Go build tags (`//go:build integration`) for integration tests?
- **Answer:**
  - Build tags tell the Go compiler to include or exclude a file based on compile flags.
  - Standard development workflows and CI pre-commit hooks run `go test ./...` which should finish in seconds without requiring external Docker services (PostgreSQL, Redis).
  - Tagging integration test files with `//go:build integration` ensures they are only executed when explicitly targeted via `go test -tags=integration ./...`.

#### Q2: What are the two main strategies for isolating database state between integration tests, and what are their trade-offs?
- **Answer:**
  1. **Transaction Rollback (`tx.Rollback()`):**
     - *How:* Start a transaction at test setup, pass the `tx` into the repository, and defer `tx.Rollback()`.
     - *Pros:* Extremely fast; no data is ever written to disk.
     - *Cons:* Fails if repository methods manage their own internal transactions, cannot test multithreaded concurrent database queries across multiple connections, and does not test connection pool lifecycles.
  2. **Table Truncate (`TRUNCATE TABLE ... CASCADE`):**
     - *How:* Execute against a live test database and run `TRUNCATE` in test teardown or before each test.
     - *Pros:* 100% faithful to production behavior. Tests actual committed data, real connection pool concurrency, and foreign key cascading.
     - *Cons:* Slightly slower than in-memory rollbacks, but easily kept $<1$s for small to medium suites.

#### Q3: Why is testing database constraints (foreign keys, unique indexes) vital in integration tests?
- **Answer:**
  - Application logic and ORMs can have bugs or race conditions. The database is the ultimate authority for data integrity.
  - Integration tests verify that:
    1. Foreign keys reject orphan records (e.g. creating a task for a non-existent user).
    2. Unique indexes prevent duplicate records (e.g. unique email) and return the expected database error code (`23505`).
    3. The application's error mapping layer (`errors.As(err, &pgErr)`) correctly intercepts and converts vendor-specific database codes into clean domain sentinel errors (`ErrDuplicateEmail`).

---

## Phase 15: Docker Multi-Stage Builds, Security & Compose Orchestration

#### Q1: What is a Multi-Stage Docker build and why is it especially beneficial for Go applications?
- **Answer:**
  - A multi-stage build uses multiple `FROM` instructions in a single `Dockerfile`. Each stage can use a different base image, and files (artifacts) can be selectively copied from one stage to another.
  - In Go, the compiler produces a self-contained static machine binary. By building in a heavy `golang:alpine` builder stage ($\sim 800\text{MB}$) and copying *only* the compiled binary into a minimal `alpine` runner stage ($\sim 5\text{MB}$), the final production image size drops by $>90\%$, eliminating all compilers, package managers, and source code from production.

#### Q2: What do `CGO_ENABLED=0` and `-ldflags="-s -w"` achieve?
- **Answer:**
  - `CGO_ENABLED=0`: Forces the Go compiler to emit a 100% statically linked binary with zero dependencies on dynamic C libraries (`glibc`/`musl`). This ensures the binary executes on any Linux kernel or minimal base image (`alpine`, `distroless`, `scratch`).
  - `-s`: Strips the symbol table (debugging symbols).
  - `-w`: Strips DWARF debug information.
  - Together, they shrink binary size by $30\text{–}40\%$ with zero runtime performance penalty.

#### Q3: Why should production containers NEVER run as `root`?
- **Answer:**
  - If a container runs as `root` (UID 0) and an attacker exploits a remote code execution (RCE) vulnerability or a Linux kernel container escape flaw, they immediately obtain root access over the host operating system.
  - Running as an unprivileged system user (`USER appuser`) adheres to the Principle of Least Privilege, containing the blast radius of any exploit.

#### Q4: What is the difference between standard `depends_on` and `depends_on` with conditions in Docker Compose?
- **Answer:**
  - Plain `depends_on`: Only checks that the target container's process has started. PostgreSQL or Redis may take 2-5 seconds to initialize data files and accept connections. If the API boots immediately, it crashes with `connection refused`.
  - `depends_on` with `condition: service_healthy`: Waits until the target service's healthcheck command (`pg_isready`, `redis-cli ping`) succeeds before starting the dependent service.
  - `depends_on` with `condition: service_completed_successfully`: Ensures one-shot batch tasks (like database schema migrations) exit with status 0 before starting the web server.
