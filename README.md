# Cloud-Native Transaction Processing Service

A production-grade, event-driven transaction processing microservice built with **Go**, provisioned on **AWS** using **Terraform**. Designed to handle high-throughput financial transactions with idempotency guarantees, distributed transaction management, and race condition prevention.

![Go](https://img.shields.io/badge/Go-1.22-00ADD8?logo=go&logoColor=white)
![Terraform](https://img.shields.io/badge/Terraform-1.7-844FBA?logo=terraform&logoColor=white)
![AWS](https://img.shields.io/badge/AWS-Cloud-FF9900?logo=amazonaws&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?logo=docker&logoColor=white)
![License](https://img.shields.io/badge/License-MIT-green)

## Architecture

```
┌──────────┐     ┌──────────────┐     ┌──────────┐     ┌────────────┐
│  Client   │────▶│  API Gateway │────▶│  Go App  │────▶│  DynamoDB  │
└──────────┘     └──────────────┘     └────┬─────┘     └────────────┘
                                           │
                                    ┌──────┴──────┐
                                    ▼             ▼
                              ┌──────────┐  ┌──────────┐
                              │   SQS    │  │   SNS    │
                              │ (Queue)  │  │ (Notify) │
                              └────┬─────┘  └──────────┘
                                   ▼
                            ┌─────────────┐
                            │ EventBridge │
                            │  (Events)   │
                            └─────────────┘
```

## Features

- **Idempotent Transaction Processing** — Duplicate requests safely handled via idempotency keys stored in DynamoDB with TTL-based expiration
- **Distributed Transaction Management** — Two-phase state machine (PENDING → PROCESSING → SETTLED / FAILED) with optimistic locking to prevent race conditions
- **Event-Driven Architecture** — SQS for async processing, SNS for fan-out notifications, EventBridge for domain event routing
- **Structured Observability** — JSON-structured logging, request tracing via correlation IDs, health check endpoints
- **Clean Architecture** — Handler → Service → Repository layering with dependency injection
- **Infrastructure as Code** — Modular Terraform with separate modules for each AWS resource

## Project Structure

```
.
├── cmd/server/             # Application entrypoint
│   └── main.go
├── internal/               # Private application code
│   ├── config/             # Configuration management
│   ├── handler/            # HTTP handlers (transport layer)
│   ├── middleware/         # HTTP middleware (logging, recovery, idempotency)
│   ├── model/              # Domain models and DTOs
│   ├── repository/         # Data access layer (DynamoDB)
│   └── service/            # Business logic layer
├── pkg/                    # Public reusable packages
│   ├── idempotency/        # Idempotency key management
│   └── logger/             # Structured logging
├── terraform/              # Infrastructure as Code
│   ├── main.tf
│   ├── variables.tf
│   ├── outputs.tf
│   └── modules/            # Reusable Terraform modules
│       ├── sqs/
│       ├── sns/
│       ├── dynamodb/
│       └── eventbridge/
├── test/                   # Integration tests
├── scripts/                # Build & deploy scripts
├── docs/                   # GitHub Pages documentation
├── .github/workflows/      # CI/CD pipeline
├── Dockerfile
├── Makefile
└── docker-compose.yml
```

## Quick Start

### Prerequisites
- Go 1.22+
- Docker & Docker Compose
- AWS CLI (configured)
- Terraform 1.7+

### Local Development

```bash
# Clone the repository
git clone https://github.com/yourusername/txn-processing-service.git
cd txn-processing-service

# Install dependencies
go mod download

# Run locally with LocalStack (mock AWS)
docker-compose up -d

# Run the application
make run

# Run tests
make test

# Build binary
make build
```

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/v1/transactions` | Submit a new transaction |
| GET | `/api/v1/transactions/:id` | Get transaction by ID |
| GET | `/api/v1/transactions` | List transactions (paginated) |
| PATCH | `/api/v1/transactions/:id/settle` | Settle a pending transaction |
| GET | `/health` | Health check |
| GET | `/ready` | Readiness check |

### Example Request

```bash
curl -X POST http://localhost:8080/api/v1/transactions \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: txn-abc-123" \
  -d '{
    "sender_id": "user_001",
    "receiver_id": "user_002",
    "amount": 150.00,
    "currency": "USD",
    "description": "Payment for services"
  }'
```

### Deploy Infrastructure

```bash
cd terraform

# Initialise Terraform
terraform init

# Preview changes
terraform plan -var="environment=dev"

# Apply infrastructure
terraform apply -var="environment=dev"
```

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `AWS_REGION` | `ap-south-1` | AWS region |
| `DYNAMODB_TABLE` | `transactions` | DynamoDB table name |
| `SQS_QUEUE_URL` | — | SQS queue URL |
| `SNS_TOPIC_ARN` | — | SNS topic ARN |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `IDEMPOTENCY_TTL` | `24h` | Idempotency key TTL |

## Testing

```bash
# Unit tests
make test

# Unit tests with coverage
make test-coverage

# Integration tests (requires LocalStack)
make test-integration

# Lint
make lint
```

## License

MIT License — see [LICENSE](LICENSE) for details.
