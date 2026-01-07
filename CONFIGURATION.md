# Configuration Examples

## Using a Configuration File

Create a `dcum.yaml` file in your project root or home directory:

```yaml
exclude_patterns:
  - "**/node_modules/**"
  - "**/.git/**"
  - "**/vendor/**"
  - "**/dist/**"
  - "**/build/**"
  - "**/target/**"
  - "**/.venv/**"
  - "**/venv/**"
```

## Using Environment Variables

You can also configure dcum using environment variables with the `DCUM_` prefix:

```bash
# Set exclusion patterns via environment variable
export DCUM_EXCLUDE_PATTERNS="**/node_modules/**,**/.git/**,**/vendor/**"

# Run dcum
./dcum
```

## Configuration Precedence

Configuration is loaded in the following order (later sources override earlier ones):

1. Default values (built into the application)
2. Configuration file (searched in order):
   - `./dcum.yaml`
   - `~/.config/dcum/dcum.yaml`
   - `~/dcum.yaml`
   - `/etc/dcum/dcum.yaml`
3. Environment variables (with `DCUM_` prefix)

## Pattern Syntax

Exclusion patterns use glob syntax:

- `*` - matches any sequence of characters (except `/`)
- `**` - matches any sequence of characters (including `/`)
- `?` - matches any single character
- `[abc]` - matches any character in the set
- `[a-z]` - matches any character in the range

### Examples

```yaml
exclude_patterns:
  # Exclude all node_modules directories
  - "**/node_modules/**"
  
  # Exclude all .git directories
  - "**/.git/**"
  
  # Exclude specific directory by name
  - "**/my-excluded-dir/**"
  
  # Exclude all directories starting with "test"
  - "**/test*/**"
  
  # Exclude all files ending with .bak
  - "**/*.bak"
```

## Testing Your Configuration

To verify your configuration is working correctly, you can:

1. Create a `dcum.yaml` file with your exclusion patterns
2. Run dcum and observe which directories are scanned
3. Check the console output for any scanning messages

The application will skip directories matching your exclusion patterns and won't scan for docker-compose files within them.
