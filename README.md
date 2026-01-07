# dcum

A terminal UI for managing docker-compose container image versions.

## Usage

Run dcum to start the UI. App will automatically find all docker-compose files in the current directory and subdirectories. All found container images will be displayed in a table. For each record you will be able to pick the version you want to upgrade to. After you have selected the versions you want to upgrade to, you can apply the changes.

## Configuration

dcum can be configured using a YAML configuration file. The configuration file can be placed in one of the following locations (in order of precedence):

1. `./dcum.yaml` (current directory)
2. `~/.config/dcum/dcum.yaml`
3. `~/dcum.yaml`
4. `/etc/dcum/dcum.yaml`

You can also use environment variables with the `DCUM_` prefix.

### Exclusion Patterns

You can exclude directories and files from scanning using glob patterns:

```yaml
exclude_patterns:
  - "**/node_modules/**"
  - "**/.git/**"
  - "**/vendor/**"
  - "**/dist/**"
  - "**/build/**"
```

Default exclusion patterns (if no config file is found):
- `**/node_modules/**`
- `**/.git/**`
- `**/vendor/**`

See `dcum.yaml.example` for a complete configuration example.

## Building

To build the project, run:

```bash
go build -o dcum ./cmd/dcum
```

To run it directly:

```bash
go run ./cmd/dcum
```
