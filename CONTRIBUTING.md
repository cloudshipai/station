# Contributing to Station 

Thank you for your interest in contributing to Station! This guide will help you get started with contributing to our revolutionary AI infrastructure platform.

## 🎯 Our Vision

Station aims to make MCP (Model Context Protocol) servers as easy to discover, configure, and deploy as Docker containers. We're building the "package manager for AI agents" that transforms complex MCP server setup from hours of configuration into a 30-second guided wizard.

## 🤖 Agentic Coding PRs Welcome!

We **encourage and welcome** Pull Requests created with AI assistance (Claude, GPT, Copilot, etc.)! However, to maintain code quality and project coherence, agentic PRs must follow these guidelines:

### ✅ Requirements for Agentic PRs

1. **Small, Focused Changes**: Break large features into multiple small PRs (ideally <300 lines changed)
2. **Passing Tests**: All existing tests must pass, and new functionality must include tests
3. **Clear Explanation**: Include a detailed explanation of:
   - What the change does and why it's needed
   - How you tested the functionality
   - Any AI tools used in development
   - Any design decisions made during implementation

4. **Documentation**: Update relevant documentation (README, code comments, etc.)

### 🎯 Great Agentic PR Examples

- **✅ Good**: "Add validation for GitHub URL format in load command with tests"
- **✅ Good**: "Implement MCP server timeout configuration with error handling"
- **❌ Too Large**: "Complete rewrite of agent execution system"
- **❌ Missing Context**: "Fix bug" (without explanation)

## 🛠️ Development Setup

### Prerequisites

- **Go 1.21+**: Required for building Station
- **SQLite3**: Database backend
- **Git**: Version control
- **Make**: Build system

### Quick Setup

```bash
# Clone the repository
git clone https://github.com/cloudshipai/station.git
cd station

# Install dependencies
go mod download

# Build development binary
make dev

# Initialize Station
./stn init

# Run tests
make test
```

### Development Workflow

1. **Fork the repository** on GitHub
2. **Create a feature branch**: `git checkout -b feature/amazing-feature`
3. **Make your changes** with tests
4. **Run tests**: `make test`
5. **Check linting**: `make lint`
6. **Commit**: `git commit -m 'Add amazing feature'`
7. **Push**: `git push origin feature/amazing-feature`
8. **Create Pull Request** with detailed description

## 🧪 Testing

We maintain high test coverage and require tests for all new functionality.

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific test
go test ./internal/services -v

# Run integration tests
make test-integration
```

### Test Standards

- **Unit Tests**: Test individual functions and methods
- **Integration Tests**: Test component interactions
- **CLI Tests**: Test command-line interface functionality
- **Coverage Target**: Maintain >80% code coverage

### Writing Tests

```go
func TestMyFunction(t *testing.T) {
    // Arrange
    input := "test input"
    expected := "expected output"
    
    // Act
    result := MyFunction(input)
    
    // Assert
    assert.Equal(t, expected, result)
}
```

## 📋 Code Standards

### Go Style Guide

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Use meaningful variable and function names
- Add comments for exported functions
- Handle errors appropriately

### Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add GitHub MCP server discovery
fix: resolve database connection timeout
docs: update installation instructions
test: add tests for config validation
chore: update dependencies
```

### Code Review Process

1. **Automated Checks**: CI must pass (tests, linting, coverage)
2. **Manual Review**: At least one maintainer review required
3. **Feedback Integration**: Address review comments promptly
4. **Final Approval**: Maintainer approval required for merge

## 🚀 Feature Development

### Adding New Features

1. **Create Issue**: Describe the feature and get feedback
2. **Design Discussion**: For large features, discuss architecture
3. **Implementation**: Create PR with tests and documentation
4. **Review**: Iterate based on feedback
5. **Merge**: Maintainer merges when ready

### Feature Guidelines

- **User-Focused**: Features should solve real user problems
- **Backward Compatible**: Don't break existing functionality
- **Well-Tested**: Include comprehensive tests
- **Documented**: Update README and relevant docs
- **Performance Conscious**: Consider impact on startup time and memory

## 🐛 Bug Fixes

### Reporting Bugs

Use our [bug report template](.github/ISSUE_TEMPLATE/bug_report.md):

- Clear description of the problem
- Steps to reproduce
- Expected vs actual behavior
- Environment details (OS, Go version, etc.)
- Relevant logs or error messages

### Fixing Bugs

1. **Reproduce**: Confirm you can reproduce the bug
2. **Write Test**: Create a failing test that demonstrates the bug
3. **Fix**: Implement the minimal fix
4. **Verify**: Ensure the test now passes
5. **Submit PR**: Include test and fix

## 📚 Documentation

### Documentation Standards

- **README**: Keep installation and usage instructions current
- **Code Comments**: Document exported functions and complex logic
- **Change Documentation**: Update docs when behavior changes
- **Examples**: Provide working examples for new features

### Writing Documentation

- Use clear, concise language
- Include code examples
- Test documentation examples
- Keep it up-to-date with code changes

## 🏷️ Release Process

Station uses automated releases via GitHub Actions and GoReleaser:

1. **Version Bump**: Update version in appropriate files
2. **Tag Release**: Create git tag following semantic versioning
3. **Automated Build**: GitHub Actions builds binaries for all platforms
4. **Package Distribution**: Binaries published to GitHub Releases
5. **One-liner Update**: Install script automatically uses latest version

## 🤝 Community

### Getting Help

- **GitHub Issues**: For bugs and feature requests
- **GitHub Discussions**: For questions and community discussion
- **Discord/Slack**: [Coming Soon] For real-time chat

### Code of Conduct

We follow the [Contributor Covenant Code of Conduct](https://www.contributor-covenant.org/version/2/1/code_of_conduct/). Be respectful, inclusive, and constructive in all interactions.

## 🎉 Recognition

Contributors will be recognized in:

- **Release Notes**: Major contributions highlighted
- **README**: Contributors section
- **GitHub**: Contributor statistics and badges

## 📞 Contact

- **Maintainers**: See [CODEOWNERS](.github/CODEOWNERS)
- **Issues**: [GitHub Issues](https://github.com/cloudshipai/station/issues)
- **Email**: [Coming Soon]

## 🙏 Thank You

Every contribution, no matter how small, helps make Station better for the entire AI community. Whether you're fixing a typo, adding a feature, or improving documentation, your efforts are deeply appreciated!

---

**Happy Coding!** 🚂✨

*Made with ❤️ by the Station community*