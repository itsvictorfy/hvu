# hvu - Helm Values Upgrade

[![Go Version](https://img.shields.io/github/go-mod/go-version/itsvictorfy/hvu)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/itsvictorfy/hvu)](https://goreportcard.com/report/github.com/itsvictorfy/hvu)

A CLI tool that safely upgrades Helm chart `values.yaml` files when moving to newer chart versions.

## Overview

When upgrading Helm charts, manually migrating your custom `values.yaml` can be error-prone. **hvu** automates this process by:

1. **Classifying** your values as customizations vs. copied defaults
2. **Preserving** your intentional changes
3. **Updating** defaults that have changed in the new chart version
4. **Flagging** unknown or deprecated keys for review

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/itsvictorfy/hvu.git
cd hvu

# Build and install
make install
```

### Pre-built Binaries

Download from the [Releases](https://github.com/itsvictorfy/hvu/releases) page.

## Quick Start

### Upgrade a Values File

```bash
hvu upgrade \
  --chart postgresql \
  --repo https://charts.bitnami.com/bitnami \
  --from 12.1.0 \
  --to 16.0.0 \
  --values ./my-values.yaml
```

### Classify Your Customizations

See which values are customizations vs. defaults:

```bash
hvu classify \
  --chart postgresql \
  --repo https://charts.bitnami.com/bitnami \
  --version 15.2.8 \
  --values ./my-values.yaml
```

## Commands

### `upgrade`

Upgrades a values file from one chart version to another.

```bash
hvu upgrade [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--chart` | Chart name (required) |
| `--repo` | Chart repository URL (required) |
| `--from` | Source chart version (required) |
| `--to` | Target chart version (required) |
| `-f, --values` | Path to your values file (required) |
| `-o, --output` | Output directory (default: `./upgrade-output`) |
| `--dry-run` | Preview changes without writing files |

**Example:**

```bash
hvu upgrade \
  --chart grafana \
  --repo https://grafana.github.io/helm-charts \
  --from 8.0.0 \
  --to 10.0.0 \
  --values ./grafana-values.yaml \
  --output ./upgraded
```

### `classify`

Analyzes a values file and classifies each key.

```bash
hvu classify [flags]
```

**Classification Categories:**
- `CUSTOMIZED` - Values you've intentionally changed
- `COPIED_DEFAULT` - Values matching chart defaults (safe to update)
- `UNKNOWN` - Keys not in chart defaults (may be obsolete)

**Example:**

```bash
hvu classify \
  --chart nginx-ingress \
  --repo https://kubernetes.github.io/ingress-nginx \
  --version 4.0.0 \
  --values ./ingress-values.yaml
```

### `version`

Displays version information.

```bash
hvu version
```

## Global Flags

| Flag | Description |
|------|-------------|
| `-v, --verbose` | Enable verbose logging |
| `-q, --quiet` | Suppress non-essential output |
| `-h, --help` | Help for any command |

## How It Works

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Your Values    │     │  Old Chart      │     │  New Chart      │
│  (values.yaml)  │     │  Defaults       │     │  Defaults       │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                       │
         └───────────┬───────────┴───────────┬───────────┘
                     │                       │
                     ▼                       ▼
              ┌──────────────┐        ┌──────────────┐
              │   Classify   │───────▶│    Merge     │
              │   Values     │        │    Values    │
              └──────────────┘        └──────┬───────┘
                                             │
                                             ▼
                                    ┌─────────────────┐
                                    │  Upgraded       │
                                    │  Values File    │
                                    └─────────────────┘
```

1. **Fetch** default values from both chart versions
2. **Classify** your values against the old defaults
3. **Merge** your customizations with the new defaults
4. **Output** an upgraded values file with preserved comments

## Development

### Prerequisites

- Go 1.21 or later
- Make

### Building

```bash
# Build for current platform
make build

# Build for all platforms
make build-all

# Run tests
make test

# Run linter
make lint
```

### Project Structure

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

### Running Tests

```bash
# Unit tests
make test

# With coverage
make test-coverage

# Integration tests (requires network)
go test -tags=integration ./test/...
```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Helm](https://helm.sh/) - The package manager for Kubernetes
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [gopkg.in/yaml.v3](https://gopkg.in/yaml.v3) - YAML processing
