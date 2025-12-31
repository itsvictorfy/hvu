# hvu

hvu (Helm Values Upgrade) - A CLI tool that helps safely upgrade Helm chart values.yaml files when moving to newer chart versions.

It analyzes differences between chart versions, classifies user customizations vs copied defaults, and generates an upgraded values file preserving your changes.

## Running the Program for Testing

### Option 1: Run directly with `go run`

```bash
go run ./cmd/hvu/main.go --help
```

### Option 2: Build and run the binary

```bash
# Build the binary
make build

# Run it
./bin/hvu --help
```

### Option 3: Install and run

```bash
# Install to your GOPATH/bin
make install

# Run from anywhere
hvu --help
```

## Available Commands

### Upgrade Command

Upgrade values file to a new chart version:

```bash
go run ./cmd/hvu/main.go upgrade \
  --chart postgresql \
  --repo https://charts.bitnami.com/bitnami \
  --from 12.1.0 --to 16.0.0 \
  --values ./my-values.yaml
```

Or with the built binary:

```bash
./bin/hvu upgrade \
  --chart postgresql \
  --repo https://charts.bitnami.com/bitnami \
  --from 12.1.0 --to 16.0.0 \
  --values ./my-values.yaml
```

### Classify Command

Show customizations vs defaults in a values file:

```bash
go run ./cmd/hvu/main.go classify \
  --chart postgresql \
  --repo https://charts.bitnami.com/bitnami \
  --version 15.2.8 \
  --values ./my-values.yaml
```

### Version Command

Display version information:

```bash
go run ./cmd/hvu/main.go version
```

## Global Flags

- `-o, --output string` - Output directory for generated files (default: ./upgrade-output)
- `-q, --quiet` - Suppress non-essential output
- `-v, --verbose` - Enable verbose logging

## Testing

### Unit Tests

```bash
make test
```

Runs `go test -race -cover ./...`

### With Coverage Report

```bash
make test-coverage
```

Generates an HTML coverage report at `coverage.html`

### Integration Tests

```bash
make test-integration
```

Runs the integration test script at `scripts/integration-test.sh`

### Run Everything (lint + test + build)

```bash
make all
```

### Direct Go Command

```bash
go test ./...
```

## Development

### Format Code

```bash
make fmt
```

### Run Linters

```bash
make lint
```

### Auto-fix Linting Issues

```bash
make lint-fix
```

### Tidy Go Modules

```bash
make mod-tidy
```

### Fetch Test Chart Data

```bash
make testdata
```

## Building

### Build for Current Platform

```bash
make build
```

### Build for Linux

```bash
make build-linux
```

### Build for macOS

```bash
make build-darwin
```

### Build for All Platforms

```bash
make build-all
```

## Cleaning

Remove build artifacts and coverage reports:

```bash
make clean
```

## Help

View all available make targets:

```bash
make help
```
