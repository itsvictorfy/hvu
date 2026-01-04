# Contributing to hvu

Thank you for your interest in contributing to hvu! This document provides guidelines and instructions for contributing.

## Code of Conduct

Please be respectful and constructive in all interactions. We aim to maintain a welcoming environment for all contributors.

## How to Contribute

### Reporting Issues

- Check existing issues to avoid duplicates
- Use a clear, descriptive title
- Provide steps to reproduce the issue
- Include relevant system information (OS, Go version, etc.)

### Suggesting Features

- Open an issue with the `enhancement` label
- Describe the use case and expected behavior
- Consider if the feature aligns with the project's goals

### Submitting Pull Requests

1. **Fork the repository** and create your branch from `main`

2. **Set up your development environment:**
   ```bash
   git clone https://github.com/YOUR_USERNAME/hvu.git
   cd hvu
   make mod-download
   ```

3. **Make your changes:**
   - Write clear, concise commit messages
   - Add tests for new functionality
   - Update documentation as needed

4. **Ensure quality:**
   ```bash
   # Format code
   make fmt

   # Run linter
   make lint

   # Run tests
   make test
   ```

5. **Submit your pull request:**
   - Reference any related issues
   - Describe what changes you made and why
   - Ensure CI passes

## Development Guidelines

### Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Keep functions focused and small
- Write descriptive variable and function names

### Testing

- Write unit tests for new functionality
- Maintain or improve code coverage
- Integration tests should be tagged with `//go:build integration`

### Commit Messages

Use clear, descriptive commit messages:

```
Add support for custom output formats

- Implement JSON output option
- Add YAML output option
- Update documentation
```

### Pull Request Guidelines

- Keep PRs focused on a single change
- Include tests for new functionality
- Update documentation if needed
- Ensure all CI checks pass

## Project Structure

```
hvu/
├── cmd/hvu/          # CLI entry point
├── pkg/
│   ├── cli/          # Command definitions
│   ├── helm/         # Helm chart interactions
│   ├── service/      # Business logic
│   └── values/       # YAML processing
├── test/             # Test files
└── testdata/         # Test fixtures
```

## Getting Help

- Open an issue for questions
- Check existing documentation
- Review closed issues for similar topics

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
