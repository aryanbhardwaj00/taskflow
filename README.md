# TaskFlow

TaskFlow is a production-style task management backend API (similar to a simplified Trello) built with Go, Gin, PostgreSQL, and Redis.

The project is structured according to clean architecture principles to demonstrate idiomatic Go patterns, secure authentication/authorization, database design, caching patterns, rate limiting, automated testing, and cloud readiness.

---

## 🛠 Tech Stack

- **Language:** Go (1.24+)
- **HTTP Framework:** Gin
- **Database:** PostgreSQL (Driver: `pgx`)
- **Cache:** Redis (`go-redis`)
- **Auth:** JWT (HMAC-SHA256) & bcrypt
- **Migrations:** golang-migrate
- **Testing:** Go `testing`, `testify`, mocks
- **Containerization:** Docker & Docker Compose
- **Cloud/Infra:** AWS (EC2, RDS, S3, ECR, ECS) & Terraform (Future Phase)

---

## 📁 Project Architecture & Layout

```text
taskflow/
├── cmd/
│   └── api/                    # Application entry point (main.go)
├── internal/                   # Private application code (not importable by external packages)
│   ├── config/                 # Environment and application configuration
│   ├── handler/                # HTTP request/response handlers (Gin)
│   ├── infra/                  # Infrastructure clients (PostgreSQL, Redis)
│   ├── middleware/             # Gin middlewares (Auth, Logging, Rate Limiting)
│   ├── model/                  # Domain entities / database models
│   ├── repository/             # Database access interfaces & implementations
│   └── service/                # Core business logic & external service coordination
├── migrations/                 # SQL database migration scripts
├── docs/                       # Architectural decisions, interview notes & roadmap
│   ├── engineering-decisions.md
│   ├── interview-notes.md
│   └── learning-progress.md
├── go.mod                      # Go module definition
├── .gitignore
└── README.md
```

---

## 🚀 Getting Started

*(Setup instructions will be documented as phases progress)*

---

## 📚 Documentation
- [Engineering Decisions](docs/engineering-decisions.md)
- [Interview Preparation Notes](docs/interview-notes.md)
- [Learning Progress](docs/learning-progress.md)
