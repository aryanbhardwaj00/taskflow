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

#### Q1: What is the difference between HTTP 401 Unauthorized and HTTP 403 Forbidden?
- **Answer:** `401 Unauthorized` means authentication failed (identity is unknown). `403 Forbidden` means identity is verified, but permissions/role are insufficient.

#### Q2: What is an IDOR vulnerability and how do you prevent it in SQL?
- **Answer:** Attackers guessing IDs to tamper with other users' resources. We prevent it by enforcing ownership in the SQL query: `WHERE id = $1 AND user_id = $2`.

---

## Phase 9: Redis Architecture & In-Memory Operations

### 1. Basic Interview Questions

#### Q1: What is Redis and how does its architecture differ from PostgreSQL?
- **Answer:**
  - **Storage:** Redis is an in-memory (RAM) key-value store; PostgreSQL is a disk/SSD-backed relational database.
  - **Latency:** Redis operations execute in sub-millisecond time ($100-500\mu\text{s}$), whereas PostgreSQL queries require 5-50ms due to disk I/O, locks, and indexing overhead.
  - **Durability:** PostgreSQL guarantees strict ACID durability. Redis is primarily used for volatile, disposable data (caches, rate limiters, session stores).

#### Q2: What is `redis.Nil` in the `go-redis` library? Why is it not a true system error?
- **Answer:** When calling `rdb.Get(ctx, key)` for a key that does not exist or has expired, Redis returns `(nil)`. The `go-redis` library returns this as `redis.Nil`. In application code, `errors.Is(err, redis.Nil)` represents a **Cache Miss**, which is normal operating behavior indicating we should fall back to PostgreSQL. It must not be treated as a database failure or internal 500 error.

#### Q3: How does TTL (Time-To-Live) work in Redis?
- **Answer:** Every key can have an associated expiration duration. Redis tracks this using passive and active eviction:
  - **Passive Eviction:** When a client accesses an expired key, Redis detects it has expired, deletes it, and returns `(nil)`.
  - **Active Eviction:** Periodically, Redis samples keys in the background and cleans up expired ones so dead memory is reclaimed even if the key is never read again.
