# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run

```bash
# Run with default dev environment
go run main.go

# Run with specific environment
go run main.go -env=prod
go run main.go -env=test

# Run with custom config file
go run main.go -config=/path/to/config.yaml

# Build
go build -o aio main.go
```

## Testing

```bash
# Run all tests
go test ./...

# Run tests in a specific package
go test ./pkg/scheduler/...

# Run a specific test
go test -run TestScheduler ./pkg/scheduler/

# Run tests with verbose output
go test -v ./...
```

## Architecture Overview

This is a Go backend service using DDD (Domain-Driven Design) with a strict layered architecture.

### Directory Structure

```
├── main.go              # Application entry point
├── app/                 # Application root - composes all modules
├── base/                # Infrastructure instances (DB, Redis, Logger, etc.) - NO business logic
├── router/              # Route registration - NO business logic
├── system/              # Business modules (each is an independent domain)
│   ├── config/          # Configuration center module
│   ├── executor/        # Task executor module
│   ├── registry/        # Service registry module
│   ├── server/          # Server management module
│   ├── shorturl/        # Short URL module
│   ├── ssl/             # SSL certificate module
│   ├── user/            # User/authentication module
│   └── workflow/        # Workflow engine module
├── pkg/                 # Reusable packages
│   ├── core/            # Core utilities (logger, security, etc.)
│   ├── db/              # Database utilities
│   ├── executor/        # Local task executor
│   ├── grpc/            # gRPC server utilities
│   ├── mq/              # Message queue abstraction
│   ├── oss/             # Object storage
│   ├── scheduler/       # Distributed scheduler
│   └── sdk/             # SDK for inter-service communication
├── utils/               # Utility functions
├── resources/           # Configuration files (YAML)
└── http/                # HTTP test files
```

### Module Structure (under system/)

Each module follows DDD structure:

```
system/<module>/
├── internal/            # Private implementation
│   ├── model/           # Domain entities
│   ├── dao/             # Data access (single model)
│   ├── service/         # Business logic (single model)
│   ├── app/             # Application service (orchestration, transactions)
│   └── facade/          # External dependency interfaces
├── api/                 # Public API
│   ├── dto/             # Data transfer objects
│   └── client/          # Client for other modules
├── external/            # HTTP/gRPC handlers
├── module.go            # Module facade
├── router.go            # Route registration
└── migrate.go           # Database migrations
```

### Layer Dependencies

**Allowed flow**: `Controller → App → Service → DAO`

| Layer | Can call | Cannot call |
|-------|----------|-------------|
| Controller | App, Service (via App) | DAO, other modules' Service |
| App | Service (same module), Facade | DAO, other Service |
| Service | DAO (same module), Facade | Other Service, App |
| DAO | DB | Service, Facade, business logic |
| Facade | External clients | Service, DAO |

**Critical rules**:
- Never import another module's `internal/` package
- Cross-module communication must go through `api/client`
- Complex transactions are managed at App layer
- Use `base.DB.Begin()/Commit()/Rollback()` for transactions

## Key Components

### Base Package (`base/`)
Holds singleton instances of infrastructure:
- `base.DB` - GORM database connection
- `base.RDB` - Redis client
- `base.Cache` - Redis cache wrapper
- `base.Logger` - Logger instance
- `base.Scheduler` - Distributed scheduler
- `base.Executor` - Local task executor
- `base.AdminAuth` / `base.UserAuth` / `base.ClientAuth` - Authentication

### Configuration
- Config files: `resources/{env}.yaml`
- Supports local file mode and config-center mode
- Environment variable `ENV` determines config file (dev/test/prod)

### HTTP Framework
- Uses Fiber (not net/http)
- Routes registered via `router.Register(app, fiberApp)`
- Auth middleware: `base.AdminAuth.RequireAdminAuth("perm:code")`

### Database
- GORM with MySQL/PostgreSQL dual support
- All models embed `common.Model` (int PK) or `common.ModelString` (string PK)
- Use `common.JSON` type for JSON fields
- Auto-migration via `db.AutoMigrate(base.DB)`

### Authentication
- Get user/admin info from context, never parse headers manually:
  - `security.GetUserID(ctx)`
  - `security.GetAdminId(ctx)`
  - `security.GetUserRoles(ctx)`

### Logging & Errors
- Use `logger.GetLogger().WithEntryName("Module")` for module loggers
- Use `errorc.NewErrorBuilder("Module")` for error builders
- Never use `fmt.Println` or `errors.New` in business code
- Error format: `err.New("message", underlyingErr).WithTraceID(ctx)`

### Async Tasks
Two executors available:
- `base.AioSDK.Executor` - Persistent, distributed, supports retry/delay
- `base.Executor` - In-memory, local only
- Never use bare `go func()` for async tasks

### Message Queue
Use `pkg/mq` abstraction, supports Redis Stream and RocketMQ.

## Module Integration

When creating a new module:
1. Define `Module` struct in `module.go` with unexported `internalApp` and exported `Client`
2. Create `NewModule()` constructor
3. Create `router.go` with `RegisterRoutes(m *Module, api, admin fiber.Router)`
4. Create `migrate.go` with `AutoMigrate(db *gorm.DB, log *logger.Log) error`
5. Register module in `app/app.go`
6. Call `RegisterRoutes` in `router/`

## Database Compatibility

Code must support both MySQL and PostgreSQL:
- No MySQL-specific types (`longtext`, `tinyint(1)`, `datetime`)
- Use `pkg/db/dialect.IsMySQL(db)` / `IsPostgres(db)` for dialect-specific SQL
- Prefer GORM cross-database APIs

## API Response Format

Use `pkg/core/result` for HTTP responses:
```go
// Success
return result.OK(ctx, data)

// Pagination
return result.OK(ctx, common.PageReturn{
    "total":   total,
    "content": list,
})

// Error (from App/Service)
return err  // Don't wrap with result.BadRequestNormal
```