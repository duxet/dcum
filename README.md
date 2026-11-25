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

## Labels

Update checks can be tuned per service with labels in your compose files:

| Label | Description |
|---|---|
| `wud.tag.include` | Only tags matching this regex are considered. |
| `wud.tag.exclude` | Tags matching this regex are ignored. |
| `wud.tag.transform` | `pattern => replacement` applied to tags before comparing versions. |
| `dcum.autoselect.max` | Highest update level selected automatically: `patch`, `minor` or `major` (default). |

The `wud.*` labels follow the [What's Up Docker](https://getwud.github.io/wud/) format.

### Limiting automatic selection

After checking for updates, dcum selects the lowest available update for each service (patch, then minor, then major). Some services, like databases, need extra care for major upgrades. Set `dcum.autoselect.max` to stop dcum from selecting those automatically:

```yaml
services:
  db:
    image: postgres:16.4
    labels:
      - 'dcum.autoselect.max=minor'
```

Higher updates are still listed: when none is selected, the New v column shows `- ↑`, and you can press Enter to cycle to the major version yourself. A value other than `patch`, `minor` or `major` turns off automatic selection for that service.

## Building

To build the project, run:

```bash
go build -o dcum ./cmd/dcum
```

To run it directly:

```bash
go run ./cmd/dcum
```
