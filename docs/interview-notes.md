# Golang Backend Interview Preparation Notes

This document contains curated interview questions, practical design questions, scenario-based challenges, common mistakes, and model answers covering each phase of building TaskFlow.

---

## Phase 0: Go Project Layout & Package Architecture

### 1. Basic Interview Questions

#### Q1: Why do Go projects use the `internal/` directory? What makes it special?
- **Answer:** The `internal/` package path is a compiler-enforced access rule introduced in Go 1.4. Code located inside `internal/` can only be imported by packages within the parent tree that roots above `internal`. External packages outside the module cannot import anything from your `internal/` folder.
- **Why it matters in interviews:** Interviewers want to know if you understand API boundary design. Putting application logic in `internal/` guarantees internal implementation details don't leak as public library code, preventing external breaking changes and preserving encapsulation.

#### Q2: What is the difference between `cmd/` and `internal/`?
- **Answer:** 
  - `cmd/` contains the entry points of your application (typically `cmd/api/main.go` or CLI commands). Files in `cmd/` should be thin wrappers: parse configuration, wire dependencies via constructors, and start the server/process. Very little business logic belongs in `cmd/`.
  - `internal/` holds the actual core domain, business services, database repositories, and HTTP handlers that make up the system.

---

### 2. Practical Implementation Questions

#### Q3: How do you prevent circular dependencies (`import cycle not allowed`) in Go?
- **Answer:**
  1. **Strict Layering:** Dependencies must flow in one direction:
     `Handler` $\rightarrow$ `Service` $\rightarrow$ `Repository` $\rightarrow$ `Model / Database`.
  2. **Accept interfaces, return structs:** A layer should declare the interface it *needs* rather than having the lower layer define what it provides. For example, `service` defines `type UserRepository interface`, and `repository` implements it.
  3. **Shared entities package (`model` or `domain`):** Pure data structs with no dependencies on handlers or services are placed in a common package imported by all layers.

---

## Phase 1: HTTP Server, Gin Routing & Graceful Shutdown

### 1. Basic Interview Questions

#### Q1: What is Graceful Shutdown, and why is it essential in containerized/cloud environments?
- **Answer:** Graceful shutdown is the process of stopping a service cleanly: the server immediately stops accepting *new* incoming connections while allowing active *in-flight* HTTP requests to finish executing up to a specified deadline (timeout context).
- **Interview Key Point:** In Kubernetes or AWS ECS, rolling deployments continuously terminate old pods/containers and start new ones. Without graceful shutdown, clients experience dropped connections, broken transactions, and HTTP 502 Bad Gateway errors.

#### Q2: What is the difference between `gin.Default()` and `gin.New()`?
- **Answer:**
  - `gin.New()` creates an empty `*gin.Engine` with zero middleware attached.
  - `gin.Default()` calls `gin.New()` under the hood and automatically attaches two default middlewares: `Logger()` and `Recovery()`.
  - In production systems, `gin.New()` is preferred because it gives engineers explicit control over the middleware stack and their execution order.

---

### 2. Practical Implementation Questions

#### Q3: Why is `router.Run(":8080")` discouraged in production Go applications?
- **Answer:** `router.Run()` wraps `http.ListenAndServe(addr, router)` using the default `&http.Server{}`. The default Go HTTP server has zero timeouts (`ReadTimeout = 0`, `WriteTimeout = 0`). If a client opens a connection and sends data agonizingly slowly (Slowloris attack), the server goroutine remains blocked forever, eventually exhausting system file descriptors. By explicitly configuring `&http.Server{}`, we enforce `ReadTimeout`, `WriteTimeout`, and `IdleTimeout`.

#### Q4: Why must the channel for `signal.Notify` be buffered (e.g. `make(chan os.Signal, 1)`)?
- **Answer:** `signal.Notify` does not block when delivering a signal. If the signal channel is unbuffered and the receiver goroutine is not actively waiting on the channel at the exact instant the signal arrives, the signal is dropped and lost. A buffer of 1 ensures the signal is safely held until the main goroutine reads it.

---

## Phase 2: Configuration Management & 12-Factor App

### 1. Basic Interview Questions

#### Q1: What is the 12-Factor App rule regarding configuration?
- **Answer:** Factor III specifies *"Store config in the environment"*. Configuration varies substantially across deploys (development, staging, production), but code does not. Configuration must be strictly separated from code, injected via environment variables rather than baked into source files or configuration artifacts committed to Git.

#### Q2: What is the difference between `os.Getenv` and `os.LookupEnv` in Go?
- **Answer:**
  - `os.Getenv(key)` returns the string value of the environment variable. If the variable is not set, it returns an empty string `""`. However, if the variable was explicitly set to empty `KEY=""`, it also returns `""`.
  - `os.LookupEnv(key)` returns two values: `(string, bool)`. The boolean indicates whether the variable actually exists in the environment, allowing you to distinguish between an unset variable and an intentionally empty variable.

---

## Phase 3: PostgreSQL Connection Pooling & Migrations

### 1. Basic Interview Questions

#### Q1: Why use `pgx` over the standard `lib/pq` driver?
- **Answer:**
  - `lib/pq` is in maintenance mode and lacks modern PostgreSQL features.
  - `pgx` (v5) communicates using PostgreSQL's binary protocol (faster serialization/deserialization), natively supports connection pooling via `pgxpool`, supports PostgreSQL-specific data types (UUID, JSONB, arrays, inet), and provides fine-grained query tracing.

#### Q2: Why is connection pool configuration critical in high-traffic Go services?
- **Answer:** Go creates a new goroutine for every incoming HTTP request. Without a connection pool with an upper bound (`MaxConns`), 5,000 concurrent requests would attempt to open 5,000 simultaneous TCP sockets to PostgreSQL. PostgreSQL forks a backend process per connection, which quickly exhausts server memory and reaches `max_connections` (typically 100). The pool throttles and queues requests safely.

#### Q3: Why use `UUID` instead of auto-incrementing `BIGSERIAL` for primary keys?
- **Answer:**
  - **Security (Enumeration Attacks):** With auto-incrementing IDs (`1, 2, 3`), malicious users can scrape customer records or guess total user count (`/api/v1/users/500`). UUIDs are cryptographically random and unguessable.
  - **Distributed Systems:** Multiple database shards or worker nodes can independently generate collision-free UUIDs locally without locking or coordinating with a central sequence generator.
  - *Trade-off:* UUIDs consume 16 bytes (vs 8 bytes for `BIGINT`) and random UUIDv4 causes slight B-tree fragmentation during heavy writes compared to sequentially ordered integers (mitigated in modern Postgres or by UUIDv7).

---

## Phase 4: Models, Repository Pattern & Compile-Time Dependency Injection

### 1. Basic Interview Questions

#### Q1: What does "Accept interfaces, return structs" mean in Go?
- **Answer:**
  - **Accept interfaces:** Functions and methods should declare interfaces as parameters (e.g. `NewAuthService(repo UserRepository)`). This keeps the consumer flexible and decoupled from specific implementations, making testing with mocks trivial.
  - **Return structs:** Constructor functions should return concrete structs or pointers to concrete structs (e.g. `func NewPostgresUserRepository(...) *PostgresUserRepository`). This gives callers complete visibility into all available methods and allows them to decide which interface to bind it to.

#### Q2: How does Dependency Injection in Go differ from Java/Spring Boot?
- **Answer:**
  - **Java/Spring Boot:** Uses reflection-based runtime DI via annotations (`@Autowired`, `@Inject`). Wiring errors are only discovered at runtime when the application boots and scans the classpath.
  - **Go:** Favors explicit, compile-time constructor DI (`NewService(repo)`). If a required dependency is missing or type-incompatible, the Go compiler refuses to build. There is zero reflection overhead and the dependency graph is 100% visible in code (`main.go`).

#### Q3: What are "Sentinel Errors" and how do they benefit layered architecture?
- **Answer:** Sentinel errors are package-level predefined error constants (e.g. `var ErrUserNotFound = errors.New("user not found")`). In the repository layer, we translate low-level database errors (like `pgx.ErrNoRows` or unique constraint `23505`) into domain sentinel errors. Upper layers (Service, Handler) can inspect them using `errors.Is(err, ErrUserNotFound)` without importing database drivers or coupling to PostgreSQL-specific error codes.

---

## Phase 5: Password Security & Bcrypt Deep Dive

### 1. Core Security Interview Questions

#### Q1: Why can't we use SHA-256 or MD5 for password storage? Why must we use Bcrypt or Argon2?
- **Answer:**
  - **Fast vs Slow Hashes:** SHA-256 and MD5 are general-purpose cryptographic hash functions designed to be **extremely fast** (a single consumer GPU can compute over 10 billion SHA-256 hashes per second). If a database is dumped, an attacker can crack typical user passwords in seconds using brute-force or dictionary attacks.
  - **Bcrypt is CPU and Memory-Hard:** Bcrypt is based on the Blowfish cipher and is deliberately **computationally expensive**. At default cost (10), computing a single bcrypt hash takes ~50ms to ~100ms. A GPU that cracks billions of SHA-256 hashes per second can only compute a few thousand bcrypt hashes per second, making brute-force mathematically prohibitive.

#### Q2: What is the anatomy of a Bcrypt hash string?
- **Answer:** A standard 60-character bcrypt string has four distinct components separated by `$`:
  ```text
  $2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy
  ├──  ├──  ──────────────────────  ───────────────────────────
  │    │            │                            │
  │    │            │                            └─ 31-character checksum/hash
  │    │            └─ 22-character salt (16 bytes base64 encoded)
  │    └─ 2-digit cost factor (10 means 2^10 = 1024 iterations)
  └─ Algorithm identifier (2a / 2b = bcrypt)
  ```

#### Q3: What is a Salt, and what attack does it prevent?
- **Answer:**
  - A salt is a cryptographically secure random sequence of bytes generated per password and stored alongside the hash.
  - **Attack Prevented:** **Rainbow Table Attacks** and precomputed hash lookups. Without a salt, two users with the password `"password123"` would have identical hashes. An attacker can precompute hashes of millions of common passwords and instantly look up stolen hashes. With unique per-user salts, every hash is unique, forcing attackers to crack each user's password individually from scratch.

#### Q4: What is the difference between a Salt and a Pepper?
- **Answer:**
  - **Salt:** Generated per-user, stored in plain text alongside the password hash in the database.
  - **Pepper:** A secret key known only to the application, stored outside the database (in environment variables or KMS / AWS Secrets Manager). The pepper is appended to the password before hashing. If the database alone is breached, attackers cannot crack hashes without the pepper from the application server.

#### Q5: How does Bcrypt defend against Timing Attacks?
- **Answer:** Bcrypt's password verification (`bcrypt.CompareHashAndPassword`) operates in **constant time** relative to the validity of the password. It does not return early on the first mismatched byte, ensuring attackers cannot deduce password characters by measuring microsecond differences in server response latency.

#### Q6: Why should we use DTOs (Data Transfer Objects) instead of returning Domain Models directly?
- **Answer:**
  - **Security (Over-Posting / Mass Assignment):** If a handler binds request JSON directly into a `model.User`, a client could pass `"role": "admin"` in the JSON and escalate privileges. A dedicated `RegisterRequest` DTO only binds `email` and `password`.
  - **Information Leakage:** Even with `json:"-"` struct tags on sensitive fields like `PasswordHash`, using a dedicated `RegisterResponse` DTO guarantees that only public, safe fields (`id`, `email`, `role`, `created_at`) are returned to the client.
