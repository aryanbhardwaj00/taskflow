<div align="center">

# ⚡ TaskFlow

**A High-Performance, Distributed Task Management REST API in Go**

[![Go Version](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?style=flat-square&logo=postgresql)](https://www.postgresql.org)
[![Redis](https://img.shields.io/badge/Redis-7-DC382D?style=flat-square&logo=redis)](https://redis.io)
[![Docker](https://img.shields.io/badge/Docker-Multi--Stage-2496ED?style=flat-square&logo=docker)](https://www.docker.com)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-Orchestrated-326CE5?style=flat-square&logo=kubernetes)](https://kubernetes.io)
[![Prometheus](https://img.shields.io/badge/Prometheus-Monitoring-E6522C?style=flat-square&logo=prometheus)](https://prometheus.io)
[![Grafana](https://img.shields.io/badge/Grafana-Dashboards-F46800?style=flat-square&logo=grafana)](https://grafana.com)
[![CI Pipeline](https://img.shields.io/badge/CI-GitHub_Actions-2088FF?style=flat-square&logo=githubactions)](https://github.com)
[![AWS](https://img.shields.io/badge/AWS-EC2_Deployed-FF9900?style=flat-square&logo=amazonwebservices)](https://aws.amazon.com)

A production-style task orchestration backend built with **Go**, **PostgreSQL**, and **Redis**, engineered according to **Clean Architecture** principles. Designed for high throughput, sub-millisecond read caching, distributed rate limiting, and zero-trust security.

</div>

---

## 🏛 System Architecture

TaskFlow enforces strict separation of concerns using **Layered Clean Architecture** and compile-time dependency injection.

```
                    ┌─────────────────────────┐
                    │    HTTP Client / Web    │
                    └───────────┬─────────────┘
                                │
                                ▼
                    ┌─────────────────────────┐
                    │  Gin HTTP Routing Layer │
                    │ - RateLimiter (Redis)   │
                    │ - AuthMiddleware (JWT)  │
                    └───────────┬─────────────┘
                                │
                                ▼
                    ┌─────────────────────────┐
                    │ Business Logic Service  │
                    │ (AuthService, TaskSvc)  │
                    └───────┬─────────┬───────┘
                            │         │
             ┌──────────────┘         └─────────────┐
             ▼                                      ▼
┌─────────────────────────┐            ┌─────────────────────────┐
│     PostgreSQL 16       │            │         Redis 7         │
│ - Durable Source of Truth│           │ - Cache-Aside (<1ms)    │
│ - pgxpool Connection Pool│           │ - Refresh Token Storage │
│ - Versioned Migrations  │            │ - Distributed Rate Limit│
└─────────────────────────┘            └─────────────────────────┘
```

---

## 🌟 Key Engineering Highlights

### 1. Advanced Redis Distributed Caching (Cache-Aside)
- **Sub-Millisecond Latency:** Read requests query Redis first (`task:<id>`), falling back to PostgreSQL on cache miss with an automatic 10-minute TTL.
- **Write-Invalidation:** Updates and deletions trigger atomic `DEL` commands rather than overwriting cache, eliminating concurrent race conditions.
- **Fail-Open Resilience:** If Redis experiences downtime, the service logs a warning and queries PostgreSQL directly without dropping requests.

### 2. Distributed Fixed-Window Rate Limiting
- **Atomic Concurrency:** Implemented as a Gin middleware leveraging Redis atomic `INCR` and `EXPIRE` operations.
- **RFC Standard Compliance:** Emits standard headers (`X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After`).
- **Throttling Rules:** Configured for 5 req/min on `/auth` routes and 10 req/min on `/tasks` routes, returning `HTTP 429 Too Many Requests`.

### 3. Dual-Token Authentication & Refresh Token Rotation (RTR)
- **Stateless Access Tokens:** 15-minute expiration HMAC-SHA256 (HS256) JWTs passed in `Authorization: Bearer <token>`.
- **Stateful Refresh Tokens:** 7-day cryptographic tokens stored in Redis (`refreshtoken:<token> -> userID`).
- **Token Theft Detection (RTR):** Single-use refresh tokens are rotated upon exchange. Attempted replay of an invalidated token triggers immediate revocation of all active sessions for that user.
- **Idempotent Logout:** Terminates Redis sessions and safely returns `200 OK` on repeated requests.

### 4. Object-Level Authorization (IDOR Defense)
- Enforces strict ownership at the SQL query boundary (`WHERE id = $1 AND user_id = $2`).
- Cross-user access attempts return `404 Not Found` rather than `403 Forbidden`, preventing resource ID enumeration attacks.
- Role-Based Access Control (`RequireRoles("admin")`) protects internal management routes.

### 5. Multi-Stage Containerization & CI/CD
- **Alpine Multi-Stage Build:** Statically compiles Go with `CGO_ENABLED=0` and strips debug symbols via `-ldflags="-s -w"`, reducing container image size from ~1 GB down to **75 MB**.
- **Defense in Depth:** Container executes under an unprivileged non-root user (`appuser:10001`).
- **Automated CI:** GitHub Actions workflow executes `go vet`, table-driven unit tests, and Docker container verification on every Pull Request.

### 6. Full-Stack Observability (Prometheus & Grafana)
- **Golden Signals Instrumentation:** Custom Gin middleware measuring request rates (`http_requests_total`), latency distribution (`http_request_duration_seconds` histogram), and active concurrency (`http_requests_in_flight` gauge).
- **Cardinality Explosion Defense:** Leverages `c.FullPath()` to record parameterized routes (`/api/v1/tasks/:id`) instead of raw UUIDs, preventing time-series memory bloat.
- **Zero-Config Visualization:** Automated Grafana provisioning on port `3000` with pre-wired Prometheus data source on port `9090`.

### 7. Cloud-Native Kubernetes Orchestration (`k8s/`)
- **Self-Healing Deployments:** Replicated Pods (`replicas: 2`) with autonomous recovery within seconds upon failure.
- **Zero-Downtime Rolling Updates:** Dual probes (`readinessProbe` & `livenessProbe`) testing `/health` ensure traffic is only routed to fully initialized instances.
- **Decoupled Service Discovery:** `ClusterIP` and `NodePort` abstraction layers routing traffic seamlessly across dynamic, ephemeral Pod IPs.
- **Noisy Neighbor Defense:** Explicit CPU and memory resource requests (`50m / 64Mi`) and limits (`200m / 128Mi`).

---

## 📁 Project Structure

```text
.
├── cmd/
│   └── api/                    # Application entry point (main.go), server timeouts & graceful shutdown
├── grafana/                    # Automated Grafana datasource provisioning
├── internal/
│   ├── config/                 # 12-Factor App environment configuration & startup validation
│   ├── handler/                # HTTP transport handlers & JSON DTO binding
│   ├── infra/                  # Infrastructure drivers (pgxpool PostgreSQL, go-redis)
│   ├── middleware/             # Gin middlewares (Auth, RateLimiter, RBAC, Prometheus metrics)
│   ├── model/                  # Domain entities (User, Task) and typed constants
│   ├── repository/             # Data access interfaces & PostgreSQL implementations
│   └── service/                # Business logic, cache-aside management & token rotation
├── k8s/                        # Kubernetes manifests (Deployments, Services, Configs)
├── migrations/                 # Idempotent versioned SQL migration scripts
├── .github/workflows/          # GitHub Actions CI pipeline
├── docker-compose.yml          # Local multi-service orchestration with health-checks
├── Dockerfile                  # Production multi-stage Alpine build
├── prometheus.yml              # Prometheus scrape configuration
└── docs/                       # Architecture Decision Records (ADRs)
```

---

## 🚀 Quick Start (Docker Compose)

### Prerequisites
- [Docker](https://docs.docker.com/get-docker/) & [Docker Compose](https://docs.docker.com/compose/)

### 1. Clone the Repository
```bash
git clone https://github.com/aryanbhardwaj00/taskflow.git
cd taskflow
```

### 2. Boot the Entire Stack
```bash
docker compose up -d --build
```
This single command:
1. Provisions **PostgreSQL 16** with persistent data volumes.
2. Provisions **Redis 7** with health check probes.
3. Executes automated **SQL migrations** via `golang-migrate`.
4. Compiles and starts the **TaskFlow Go API** on `http://localhost:8080`.
5. Starts **Prometheus** on `http://localhost:9090` (scraping metrics every 15s).
6. Starts **Grafana** on `http://localhost:3000` (pre-provisioned with Prometheus).

### 3. Verify Health & Observability
```bash
# Verify API liveliness
curl -i http://localhost:8080/health

# Verify Prometheus metrics export
curl -s http://localhost:8080/metrics | grep http_requests

# Open Grafana Dashboards
# http://localhost:3000 (User: admin / Pass: admin)
```

---

## ☸️ Kubernetes Deployment (Local Minikube)

TaskFlow is production-ready for deployment to Kubernetes:

```bash
# 1. Start local cluster
minikube start --driver=docker

# 2. Sync local Docker image
minikube image load taskflow-api:latest

# 3. Apply all manifests
kubectl apply -f k8s/

# 4. Verify self-healing pods & services
kubectl get pods -w
kubectl get svc

# 5. Port-forward to access the cluster
kubectl port-forward svc/taskflow-api 8080:8080
```

---

## 🧪 Testing Suite

TaskFlow incorporates a dual-layer testing strategy:

### 1. Run In-Memory Unit Tests & Mocks
Fast, deterministic tests executing in milliseconds without external dependencies:
```bash
go test -v ./internal/service/... ./internal/middleware/...
```

### 2. Run Live PostgreSQL Integration Tests
Requires PostgreSQL running. Uses Go build tags and table truncate isolation:
```bash
go test -tags=integration -v ./internal/repository/...
```

---

## 📖 API Reference

### Authentication Endpoints
| Method | Endpoint | Auth | Description |
|---|---|---|---|
| `POST` | `/api/v1/auth/register` | Public | Register user (`email`, `password` $\ge 8$ chars) |
| `POST` | `/api/v1/auth/login` | Public | Authenticate and obtain dual tokens |
| `POST` | `/api/v1/auth/refresh` | Public | Exchange refresh token for a rotated token pair |
| `POST` | `/api/v1/auth/logout` | Public | Idempotently invalidate active session in Redis |
| `GET` | `/api/v1/auth/me` | Bearer | Retrieve authenticated user profile |

### Task Management Endpoints
| Method | Endpoint | Auth | Description |
|---|---|---|---|
| `POST` | `/api/v1/tasks` | Bearer | Create a new task (`title`, `status`, `priority`) |
| `GET` | `/api/v1/tasks` | Bearer | List user tasks (`?status=todo&limit=10&offset=0`) |
| `GET` | `/api/v1/tasks/:id` | Bearer | Get task by ID (Cached in Redis) |
| `PATCH` | `/api/v1/tasks/:id/status` | Bearer | Update task status & invalidate Redis cache |
| `DELETE` | `/api/v1/tasks/:id` | Bearer | Delete task & evict from Redis cache |

### Admin Endpoints
| Method | Endpoint | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/admin/stats` | Admin | Role-gated management portal (`HTTP 403` if unauthorized) |

---

## 📐 Architecture Decisions (ADRs)

Key architectural trade-offs and decisions are formally recorded in:
- [Engineering Decisions Log](docs/engineering-decisions.md)
