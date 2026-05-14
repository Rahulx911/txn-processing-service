# Contributing

Thank you for considering contributing to the Transaction Processing Service!

## Development Setup

1. **Fork and clone** the repository
2. Install Go 1.22+ and Docker
3. Run `docker-compose up -d` to start LocalStack
4. Run `make test` to verify everything works

## Code Standards

- Follow idiomatic Go conventions (`gofmt`, `go vet`)
- Write table-driven tests for new functionality
- Keep functions focused — single responsibility
- Add structured logging for observability
- Document exported types and functions

## Pull Request Process

1. Create a feature branch from `main`
2. Write tests for new functionality
3. Ensure `make test` and `make lint` pass
4. Update documentation if adding new endpoints or config
5. Submit a PR with a clear description of changes

## Commit Messages

Follow conventional commits:
- `feat:` new features
- `fix:` bug fixes
- `refactor:` code restructuring
- `test:` adding tests
- `docs:` documentation changes
- `chore:` maintenance tasks
