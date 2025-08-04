# Contributing to Station

Thank you for considering contributing to Station! This document provides guidelines and best practices for developers working on the codebase.

## 🔧 Development Guidelines

### Database Queries - SQLC Only

**🚫 NEVER use inline SQL queries in Go code**

Station uses [SQLC](https://sqlc.dev/) for type-safe, generated database access. All SQL queries must be defined in `.sql` files and generated through SQLC.

#### ❌ Wrong (Inline SQL):
```go
// DON'T DO THIS
func (r *Repository) GetUser(id int64) (*User, error) {
    query := `SELECT id, name, email FROM users WHERE id = ?`
    var user User
    err := r.db.QueryRow(query, id).Scan(&user.ID, &user.Name, &user.Email)
    return &user, err
}
```

#### ✅ Correct (SQLC):
```sql
-- internal/db/queries/users.sql
-- name: GetUser :one
SELECT id, name, email FROM users WHERE id = ?;
```

```go
// Repository uses generated SQLC methods
func (r *Repository) GetUser(id int64) (*User, error) {
    sqlcUser, err := r.queries.GetUser(context.Background(), id)
    if err != nil {
        return nil, err
    }
    return convertFromSQLCUser(sqlcUser), nil
}
```

#### Why SQLC?
- **Type Safety**: Compile-time validation of SQL queries
- **Performance**: No reflection, direct struct mapping
- **Maintainability**: SQL changes are caught at build time
- **Consistency**: Standardized database access patterns
- **Testability**: Easy to mock generated interfaces

#### Adding New Queries:
1. **Add SQL to appropriate `.sql` file** in `internal/db/queries/`
2. **Run SQLC generation**: `sqlc generate`
3. **Use generated methods** in repository layer
4. **Add conversion functions** between SQLC types and domain models

#### Example Workflow:
```bash
# 1. Add query to SQL file
echo "-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = ?;" >> internal/db/queries/users.sql

# 2. Generate SQLC code
sqlc generate

# 3. Use in repository
func (r *UserRepo) GetByEmail(email string) (*models.User, error) {
    user, err := r.queries.GetUserByEmail(context.Background(), email)
    if err != nil {
        return nil, err
    }
    return convertUserFromSQLc(user), nil
}
```

### File-Based Configuration

- **Use XDG Base Directory Specification**: `~/.config/station/`
- **Environment Isolation**: Separate configs per environment
- **Template Variables**: Use `{{VARIABLE}}` placeholders for GitOps
- **Variable Hierarchy**: template-specific → global → env vars → interactive

### Error Handling

- **Wrap errors with context**: `fmt.Errorf("failed to do X: %w", err)`
- **Use meaningful error messages** that help debugging
- **Don't expose internal details** in user-facing errors

### Testing

- **Unit tests** for business logic
- **Integration tests** for database operations
- **E2E tests** for CLI commands
- **Use testify/assert** for assertions

### Code Organization

- **Repository Pattern**: Database access only in `internal/db/repositories/`
- **Service Layer**: Business logic in `internal/services/`
- **Handler Layer**: CLI/API handlers in `cmd/main/handlers/`
- **Models**: Domain models in `pkg/models/`

## 🚀 Getting Started

### Prerequisites
- Go 1.21+
- SQLite3
- SQLC (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`)

### Development Setup
```bash
# Clone repository
git clone https://github.com/cloudshipai/station.git
cd station

# Install dependencies
go mod download

# Generate SQLC code
sqlc generate

# Build
make build

# Run tests
make test
```

### Making Changes

1. **Create feature branch**: `git checkout -b feature/your-feature`
2. **Make changes following guidelines above**
3. **Add tests** for new functionality
4. **Ensure SQLC compliance** - no inline SQL queries
5. **Run tests**: `make test`
6. **Commit with descriptive messages**
7. **Push and create PR**

### Commit Messages

Use conventional commits format:
- `feat:` - New features
- `fix:` - Bug fixes  
- `refactor:` - Code refactoring
- `docs:` - Documentation changes
- `test:` - Test additions/modifications

Example:
```
feat: add support for PostgreSQL MCP server

- Add PostgreSQL query templates with {{variables}}
- Implement connection pooling configuration
- Add integration tests for database operations

🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

## 🔍 Code Review Guidelines

### For Reviewers:
- **Check for inline SQL queries** - ensure SQLC compliance
- **Verify error handling** - proper wrapping and context
- **Security review** - no hardcoded secrets or credentials
- **Test coverage** - adequate tests for new functionality

### For Contributors:
- **Self-review** before submitting PR
- **Add screenshots** for UI changes
- **Document breaking changes** in PR description
- **Update documentation** for new features

## 📚 Architecture Overview

Station follows a layered architecture:

```
┌─────────────────────────────────────────┐
│ CLI/TUI/API Handlers                    │
│ (cmd/main/handlers/)                    │
├─────────────────────────────────────────┤
│ Service Layer                           │
│ (internal/services/)                    │
├─────────────────────────────────────────┤
│ Repository Layer (SQLC Generated)      │
│ (internal/db/repositories/)             │
├─────────────────────────────────────────┤
│ Database Layer                          │
│ (SQLite with migrations)                │
└─────────────────────────────────────────┘
```

## 🛡️ Security Guidelines

- **Never commit secrets** or credentials
- **Use environment variables** for sensitive configuration
- **Validate all user inputs** 
- **Follow principle of least privilege**
- **Audit logging** for sensitive operations

## 📖 Documentation

- **Update README.md** for user-facing changes
- **Add inline documentation** for complex functions
- **Update API documentation** for new endpoints
- **Create examples** for new MCP templates

## 🤝 Community

- **Be respectful** in all interactions
- **Help newcomers** get started
- **Share knowledge** through documentation and examples
- **Report security issues** privately to maintainers

## 🎯 Focus Areas

Current development priorities:
1. **SQLC Migration**: Eliminate all remaining inline queries
2. **MCP Template Library**: Expand supported integrations  
3. **Agent Intelligence**: Better tool selection algorithms
4. **Performance**: Optimize tool discovery and execution
5. **Security**: Enhanced audit logging and access controls

---

Thank you for contributing to Station! Together we're building the future of intelligent AI automation. 🚀