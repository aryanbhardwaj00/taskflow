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
