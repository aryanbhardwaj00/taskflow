# TaskFlow — Learning & Implementation Progress

This document tracks our roadmap, current status, manual implementations completed, and interview topics mastered.

---

## 🗺️ Roadmap & Phase Tracker

| Phase | Description | Status | Manual Coding Focus | Interview Focus |
|---|---|---|---|---|
| **Phase 0** | Repository initialization & folder structure | 🟢 Completed | Project structure & ADR setup | Go project layout, `internal/` package rule, circular dependency prevention |
| **Phase 1** | Go module, application entry point & Gin server | 🟢 Completed | Gin routing, Graceful shutdown logic | HTTP lifecycle, Gin engine vs router group, signal handling, server timeouts |
| **Phase 2** | Environment configuration & app startup | 🟢 Completed | Config loading, validation logic, `.env` parsing | `os.Getenv` vs typed config structs, 12-Factor App principles, default fallbacks |
| **Phase 3** | PostgreSQL connection & migrations | 🟢 Completed | Connection pool configuration, ping verification, SQL schemas | Connection pool sizing (`MaxConns`, `IdleConns`), UUID vs BIGINT, TIMESTAMPTZ, migration strategies |
| **Phase 4** | Models, repository interfaces & dependency injection | 🟢 Completed | Repository interfaces, constructor functions | "Accept interfaces, return structs", Compile-time DI |
| **Phase 5** | User registration & password hashing | 🟢 Completed | bcrypt hashing & verification, User service & handler | Password storage security, salt, work factor, timing attacks, DTO protection |
| **Phase 6** | Login & JWT generation | 🟡 In Progress | JWT claims construction, HS256 signing, Login service & handler | JWT structure (header/payload/signature), symmetric vs asymmetric keys, signing algorithms |
| **Phase 7** | JWT validation & authentication middleware | ⚪ Pending | Gin Auth middleware, `c.Set`/`c.Get`, error handling | Token tampering, signature verification, `c.Abort()` vs `c.Next()` |
| **Phase 8** | Protected APIs & role-based authorization | ⚪ Pending | Task CRUD operations, ownership verification | Authentication vs Authorization, RBAC design |
| **Phase 9** | Redis installation, connection & basic ops | ⚪ Pending | Redis client initialization, ping/healthcheck | Redis architecture, in-memory storage vs disk, data types |
| **Phase 10** | Redis caching & cache invalidation | ⚪ Pending | Cache-aside pattern, cache invalidation | Cache-aside, cache penetration, breakdown, stampede, TTL strategies |
| **Phase 11** | Rate limiting using Redis | ⚪ Pending | Token bucket / Fixed window rate limiter middleware | DoS mitigation, 429 Too Many Requests, distributed rate limiting |
| **Phase 12** | Refresh token handling & rotation | ⚪ Pending | Refresh token generation, storage & rotation | Short-lived vs long-lived tokens, token revocation, replay attacks |
| **Phase 13** | Unit tests & mocks | ⚪ Pending | Table-driven tests, `testify/mock` implementations | Unit vs Integration testing, mocking best practices |
| **Phase 14** | Integration tests | ⚪ Pending | Test environment setup, real DB & Redis tests | Test isolation, rollback strategies, dockerized test DBs |
| **Phase 15** | Dockerfile & Docker Compose | ⚪ Pending | Multi-stage Dockerfile, compose service definitions | Container optimization, non-root users, layer caching |
| **Phase 16** | Logging, monitoring & structured error handling | ⚪ Pending | Structured logger (slog/zap), custom error types | Distributed tracing, log levels, centralized error unwrapping |
| **Phase 17** | Performance optimization & concurrency | ⚪ Pending | Worker pools / goroutines for background tasks | Go runtime scheduler (GMP), goroutine leaks, channels vs sync.Mutex |
| **Phase 18** | AWS deployment | ⚪ Pending | AWS configuration, container registry pushing | Cloud native deployments, VPC, ECS, RDS managed services |
| **Phase 19** | CI/CD & Terraform | ⚪ Pending | GitHub Actions workflow, Terraform definitions | Infrastructure as Code, automated deployment pipelines |

---

## 📝 Learning Log Notes
- *Phase 0 completed on:* 2026-10-31
- *Phase 1 completed on:* 2026-10-31 (Health check endpoint `/health` & `/api/v1/health`, custom timeouts, graceful shutdown with OS signals)
- *Phase 2 completed on:* 2026-10-31 (Centralized typed configuration in `internal/config`, `godotenv`, fail-fast validation, environment overrides)
- *Phase 3 completed on:* 2026-10-31 (`pgx/v5` connection pool, Docker Compose PostgreSQL service, versioned migrations for `users` and `tasks` with indexes and foreign keys)
- *Phase 4 completed on:* 2026-10-31 (Domain models `User` & `Task`, `UserRepository` interface & Postgres implementation, constructor dependency injection)
- *Phase 5 completed on:* 2026-10-31 (User registration endpoint, bcrypt password hashing with default cost 10, input normalization, Gin DTO validation, HTTP 201/409/400 mapping)
