# Contributing to Station 🚂

Welcome to the Station contributing documentation! This directory contains detailed guides for contributors.

## 📚 Documentation Index

### **[TESTING.md](./TESTING.md)**
Comprehensive testing guide covering:
- Unit tests and integration tests
- CLI command testing
- Server and SSH testing
- Performance benchmarks
- Test automation scripts
- Debugging common issues

### **[../CONTRIBUTING.md](../../CONTRIBUTING.md)**
Main contributing guide covering:
- Development setup
- Agentic PR guidelines
- Code standards
- Review process

## 🚀 Quick Start for Contributors

1. **Setup Development Environment**
   ```bash
   git clone https://github.com/cloudshipai/station.git
   cd station
   make setup
   ```

2. **Run Tests**
   ```bash
   ./test-all.sh
   ```

3. **Start Development**
   ```bash
   make dev
   ./stn init
   ./stn serve
   ```

## 📋 Contributing Checklist

- [ ] Read [CONTRIBUTING.md](../../CONTRIBUTING.md)
- [ ] Follow [TESTING.md](./TESTING.md) for testing
- [ ] Ensure tests pass: `make test`
- [ ] Check code quality: `make lint`
- [ ] Test CLI functionality manually
- [ ] Update documentation if needed

## 🤖 Agentic Contributions Welcome!

We encourage AI-assisted development! See our [agentic PR guidelines](../../CONTRIBUTING.md#-agentic-coding-prs-welcome) for:
- Small, focused changes
- Passing tests
- Clear explanations
- Proper documentation

## 🛠️ Development Tools

- **Build**: `make dev` or `make build`
- **Test**: `make test` or `./test-all.sh`
- **Lint**: `make lint`
- **Clean**: `make clean`
- **Help**: `make help`

## 📞 Getting Help

- **GitHub Issues**: Bug reports and feature requests
- **GitHub Discussions**: Questions and community discussion
- **Documentation**: Check `/docs/` for guides

---

**Happy Contributing!** 🎉

*Station is built by the community, for the community. Every contribution makes it better.*