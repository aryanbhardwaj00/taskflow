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

### 3. Scenario-Based Questions

#### Scenario: "Your junior engineer places database queries directly inside Gin handler functions to quickly deliver a feature. What issues arise, and how would you explain the refactoring needed?"
- **How to explain this concisely:**
  - *"Tight coupling between the transport (HTTP) and the persistence layer (SQL) breaks the Single Responsibility Principle."*
  - **Issues created:**
    1. **Untestable:** Testing the handler requires spinning up a live PostgreSQL database instead of mocking queries.
    2. **Code duplication:** If a gRPC endpoint, CLI tool, or background worker needs the same query, it cannot reuse the handler logic.
    3. **Difficult migration/caching:** Adding a Redis cache requires modifying HTTP code.
  - **Refactoring:** Extract the SQL into `repository`, put domain validation into `service`, and keep `handler` responsible strictly for parsing HTTP requests, status codes, and JSON serialization.

---

### 4. Common Mistakes to Avoid
- **Mistake 1: Giant `utils` package:** Dumping unrelated helpers into `pkg/utils` or `internal/utils`. Idiomatic Go favors cohesive packages (e.g. `hash`, `validator`, `clock`).
- **Mistake 2: Premature over-abstraction:** Creating interfaces for everything when there is only one implementation and no mocking needed. (Mocking unit tests is the primary reason we use interfaces for repositories and services).
- **Mistake 3: Global state / singletons:** Initializing database connections as package-level global variables (`var DB *pgxpool.Pool`) rather than passing dependencies via struct constructors.

---

### 5. Follow-Up Questions to Expect
- *"How does Go's dependency injection compare to Java's Spring Boot (reflection/runtime DI vs. compile-time constructor injection)?"*
- *"When would you put code into `pkg/` instead of `internal/`?"*
