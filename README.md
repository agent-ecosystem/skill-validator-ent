# skill-validator-ent

Enterprise CLI for validating, analyzing, and scoring Agent Skill packages using AWS Bedrock.

This tool wraps the [skill-validator](https://github.com/agent-ecosystem/skill-validator) library and replaces direct Anthropic/OpenAI API access with AWS Bedrock's Converse API, so teams can run LLM-as-judge scoring using their existing AWS credentials.

Future development may add other providers as needed.

## Prerequisites

- Go 1.25.5+
- AWS CLI v2 (for credential setup)
- An AWS account with Bedrock model access enabled

## Installation

### From source

```bash
go install github.com/agent-ecosystem/skill-validator-ent/cmd/skill-validator-ent@latest
```

### Homebrew

```bash
brew tap agent-ecosystem/homebrew-tap
brew install skill-validator-ent
```

## Command Reference

For the full command reference, refer to the base `skill-validator`
[README](https://github.com/agent-ecosystem/skill-validator/blob/main/README.md).

## AWS Authentication Setup

You need valid AWS credentials before running any `score evaluate` commands. The tool uses the standard AWS SDK credential chain.

### Already have AWS configured?

You may already have a working profile from another tool. Check with:

```bash
# List all profiles in your AWS config
aws configure list-profiles

# Show details for a specific profile
aws configure list --profile <profile-name>

# See the full config file directly
cat ~/.aws/config
```

If you find an existing profile with Bedrock access, skip to [Score with LLM judge](#score-with-llm-judge-bedrock) and use `--profile <your-profile>`.

### Option 1: AWS IAM Identity Center (SSO)

This is the most common setup for enterprise teams.

```bash
# One-time setup — creates a named profile
aws configure sso --profile bedrock
```

You'll be prompted for:
- **SSO session name**: any name (e.g. `my-sso`)
- **SSO start URL**: your org's URL (e.g. `https://your-org.awsapps.com/start`) — ask your AWS admin
- **SSO region**: the region where your Identity Center is hosted (e.g. `us-east-1`)
- **Account and role**: select from the list after browser login

Then authenticate:

```bash
aws sso login --profile bedrock
```

Use the profile with skill-validator-ent:

```bash
skill-validator-ent score evaluate path/to/skill/ \
  --model us.anthropic.claude-sonnet-4-5-20250929-v1:0 \
  --profile bedrock \
  --region us-east-1
```

### Option 2: IAM access keys

If you have long-lived access keys:

```bash
aws configure --profile bedrock
```

You'll be prompted for Access Key ID, Secret Access Key, and default region.

### Option 3: Environment variables

```bash
export AWS_ACCESS_KEY_ID=AKIA...
export AWS_SECRET_ACCESS_KEY=...
export AWS_SESSION_TOKEN=...    # only needed for temporary credentials
export AWS_REGION=us-east-1
```

### Option 4: EC2 instance profile / ECS task role

If running on AWS infrastructure, credentials are provided automatically via IMDS or the ECS credential endpoint. Just pass `--region`.

### Verifying access

Test that your credentials work and the model is accessible:

```bash
aws bedrock-runtime converse \
  --model-id us.anthropic.claude-sonnet-4-5-20250929-v1:0 \
  --messages '[{"role":"user","content":[{"text":"Hello"}]}]' \
  --region us-east-1 \
  --profile bedrock
```

If this fails, check:
1. Your IAM role has `bedrock:InvokeModel` permission
2. The model is enabled in your Bedrock console for the target region
3. Your SSO session hasn't expired (`aws sso login --profile bedrock` to refresh)

## Usage

### Validate skill structure

```bash
skill-validator-ent validate structure path/to/skill/
skill-validator-ent validate links path/to/skill/
```

### Analyze content

```bash
skill-validator-ent analyze content path/to/skill/
skill-validator-ent analyze contamination path/to/skill/
```

### Run all checks

```bash
skill-validator-ent check path/to/skill/
```

Use `--only` or `--skip` to select check groups (`structure`, `links`, `content`, `contamination`):

```bash
skill-validator-ent check --only structure,content path/to/skill/
```

### Score with LLM judge (Bedrock)

```bash
# Score a single skill
skill-validator-ent score evaluate path/to/skill/ \
  --model us.anthropic.claude-sonnet-4-5-20250929-v1:0 \
  --profile bedrock \
  --region us-east-1

# Score only SKILL.md (skip references)
skill-validator-ent score evaluate path/to/skill/ \
  --model us.anthropic.claude-sonnet-4-5-20250929-v1:0 \
  --skill-only \
  --profile bedrock \
  --region us-east-1

# Score only reference files
skill-validator-ent score evaluate path/to/skill/ \
  --model us.anthropic.claude-sonnet-4-5-20250929-v1:0 \
  --refs-only \
  --profile bedrock \
  --region us-east-1

# Re-score (overwrite cached results)
skill-validator-ent score evaluate path/to/skill/ \
  --model us.anthropic.claude-sonnet-4-5-20250929-v1:0 \
  --rescore \
  --profile bedrock \
  --region us-east-1

# Send full content (default truncates to 8,000 chars)
skill-validator-ent score evaluate path/to/skill/ \
  --model us.anthropic.claude-sonnet-4-5-20250929-v1:0 \
  --full-content \
  --profile bedrock \
  --region us-east-1
```

### View cached scores

```bash
# Show most recent scores
skill-validator-ent score report path/to/skill/

# List all cached entries
skill-validator-ent score report --list path/to/skill/

# Compare across models
skill-validator-ent score report --compare path/to/skill/

# Filter by model
skill-validator-ent score report --model us.anthropic.claude-sonnet-4-5-20250929-v1:0 path/to/skill/
```

### Score evaluate flags

| Flag | Default | Description |
|------|---------|-------------|
| `--model` | (required) | Bedrock model ID (can be set via [config file](#configuration-file)) |
| `--provider` | `bedrock` | LLM provider (only `bedrock` supported) |
| `--region` | from AWS config | AWS region override |
| `--profile` | from AWS config | AWS shared config profile override |
| `--max-response-tokens` | `500` | Max tokens in LLM response |
| `--rescore` | `false` | Overwrite cached results |
| `--skill-only` | `false` | Score only SKILL.md |
| `--refs-only` | `false` | Score only reference files |
| `--display` | `aggregate` | Reference display: `aggregate` or `files` |
| `--full-content` | `false` | Send full content (no truncation) |

### Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `-o, --output` | `text` | Output format: `text`, `json`, or `markdown` |
| `--emit-annotations` | `false` | Emit GitHub Actions `::error`/`::warning` annotations |
| `-v, --verbose` | `false` | Enable verbose/debug logging to stderr |

## Configuration file

You can persist default flag values in a YAML config file so you don't have to pass `--model`, `--region`, `--profile`, etc. on every invocation. CLI flags always override config file values.

### Config file locations

The CLI looks for config files in two places (in order of precedence):

1. **Project-level** — `.skill-validator-ent.yaml` in the current directory or any parent up to the git root
2. **User-level** — resolved in this order:
   - `$SKILL_VALIDATOR_CONFIG_DIR/config.yaml` (if the env var is set)
   - `$XDG_CONFIG_HOME/skill-validator-ent/config.yaml` (if the env var is set)
   - `~/.config/skill-validator-ent/config.yaml` (default)

Project config values override user config values. CLI flags override both.

### Example config file

```yaml
# ~/.config/skill-validator-ent/config.yaml
model: us.anthropic.claude-sonnet-4-5-20250929-v1:0
region: us-east-1
profile: bedrock
```

With this config, you can run:

```bash
# No need for --model, --region, --profile
skill-validator-ent score evaluate path/to/skill/
```

Override any value per-invocation with CLI flags:

```bash
# Use a different model for this run
skill-validator-ent score evaluate path/to/skill/ \
  --model us.anthropic.claude-haiku-4-5-20251001-v1:0
```

### Supported config keys

| Key | Equivalent flag |
|-----|----------------|
| `model` | `--model` |
| `region` | `--region` |
| `profile` | `--profile` |
| `provider` | `--provider` |
| `max_response_tokens` | `--max-response-tokens` |
| `display` | `--display` |
| `full_content` | `--full-content` |
| `output` | `--output` |
| `emit_annotations` | `--emit-annotations` |
| `strict` | `--strict` |

### Project-level config

Place a `.skill-validator-ent.yaml` at your repo root to share defaults across a team:

```yaml
# .skill-validator-ent.yaml (commit this to your repo)
model: us.anthropic.claude-sonnet-4-5-20250929-v1:0
region: us-east-1
output: json
strict: true
```

### Verbose / debug logging

Use `--verbose` (or `-v`) to see which config files were loaded and other debug info:

```bash
skill-validator-ent --verbose score evaluate path/to/skill/
```

You can also set the `SKILL_VALIDATOR_DEBUG=1` environment variable for the same effect.

## Output formats

All commands support `-o json` for machine-readable output and `-o markdown` for CI summaries:

```bash
skill-validator-ent check path/to/skill/ -o json
skill-validator-ent score evaluate path/to/skill/ --model ... -o markdown
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | No errors, no warnings |
| 1 | Validation errors present |
| 2 | Warnings present, no errors |
| 3 | CLI/usage error |

Use `--strict` (on `validate structure` and `check`) to treat warnings as errors (exit 1 instead of 2).

## Differences from base skill-validator

| Feature | skill-validator | skill-validator-ent |
|---------|----------------|---------------------|
| LLM providers | Anthropic, OpenAI (direct API) | AWS Bedrock only |
| Authentication | API keys via env vars | AWS credentials (SSO, keys, instance roles) |
| `--base-url` flag | Yes | No |
| `--max-tokens-style` flag | Yes | No |
| `--region` / `--profile` flags | No | Yes |

All non-LLM commands (validate, analyze, check) behave identically.

## Development

```bash
# Run unit tests
go test -race ./... -count=1

# Build
go build -o skill-validator-ent ./cmd/skill-validator-ent

# Lint
golangci-lint run

# Cross-compile (static binary)
CGO_ENABLED=0 go build -o skill-validator-ent ./cmd/skill-validator-ent
```

### Integration tests

There are two sets of integration tests, each gated so they don't run by default.

**Binary integration tests** build the real binary and run it as a subprocess,
verifying exit codes and output end-to-end. No AWS credentials needed.

```bash
go test -tags integration -race -v -count=1 .
```

**Bedrock integration tests** make real Converse API calls to verify the
client works against a live model. These require AWS credentials and are
gated behind an environment variable:

```bash
BEDROCK_INTEGRATION_TEST=1 \
BEDROCK_MODEL=us.anthropic.claude-sonnet-4-5-20250929-v1:0 \
BEDROCK_REGION=us-east-1 \
BEDROCK_PROFILE=<your-profile> \
go test -v ./internal/bedrock/ -run Integration -count=1
```

To run everything together:

```bash
# Unit + binary integration (no AWS needed)
go test -tags integration -race ./... -count=1

# Add Bedrock integration
BEDROCK_INTEGRATION_TEST=1 \
BEDROCK_MODEL=us.anthropic.claude-sonnet-4-5-20250929-v1:0 \
BEDROCK_REGION=us-east-1 \
BEDROCK_PROFILE=<your-profile> \
go test -tags integration -race -v ./... -count=1
```
