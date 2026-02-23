# tf-plan-tui

**An interactive terminal UI for Terraform plan output** — Think k9s for Terraform plans.

Transform wall-of-text `terraform plan` output into a navigable, filterable, searchable interface.

## Features

- 🎯 **Interactive Navigation** — Browse resources with vim-style keybindings (j/k/g/G)
- 🔍 **Filter & Search** — Filter by action type (create/update/delete/replace), search by resource name
- 📊 **Module Grouping** — Organize resources by module with expand/collapse
- ⚠️  **Risk Analysis** — Automatic risk scoring based on destructive operations and security changes
- 🔒 **Security Highlighting** — Flags potentially dangerous changes (0.0.0.0/0 CIDR, public access, encryption disabled)
- 📤 **Export** — Generate markdown summaries for PR comments
- ⚡ **Fast** — Instant startup, smooth scrolling, handles 100+ resource changes
- 🎨 **Adaptive Colors** — Works on both light and dark terminals

## Installation

### From Source

```bash
go install github.com/hashicorp/tf-plan-tui@latest
```

### Build Locally

```bash
git clone https://github.com/hashicorp/tf-plan-tui.git
cd tf-plan-tui
make install
```

### Binaries

Download from [releases](https://github.com/hashicorp/tf-plan-tui/releases).

## Quick Start

```bash
# From a saved plan file
terraform plan -out=tfplan
tf-plan-tui --plan tfplan

# From JSON
terraform show -json tfplan > plan.json
tf-plan-tui --json plan.json

# Pipe from stdin
terraform show -json tfplan | tf-plan-tui -

# Auto-run mode (runs terraform plan for you)
tf-plan-tui
```

## Usage

### Interactive Mode (TUI)

```bash
tf-plan-tui [file]                  # Launch interactive TUI
tf-plan-tui --json plan.json        # From JSON file
tf-plan-tui --plan tfplan           # From binary plan file
tf-plan-tui --filter create         # Pre-filter to creates only
tf-plan-tui --dir ./infra/prod      # Run in specific directory
```

#### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate up/down |
| `g/G` | Jump to top/bottom |
| `Ctrl+U/D` | Half-page scroll |
| `Tab` | Switch between resource list and detail pane |
| `Enter` | Expand/collapse module (when grouped) |
| `f` | Cycle through filters |
| `0-4` | Quick filter (0=all, 1=create, 2=update, 3=delete, 4=replace) |
| `/` | Search (type to filter, Esc to cancel) |
| `m` | Toggle module grouping |
| `r` | Show risk analysis |
| `e` | Export as markdown |
| `?` | Show help |
| `q` or `Ctrl+C` | Quit |

### Non-Interactive Modes

#### Export for PR Comments

```bash
# Generate markdown summary
tf-plan-tui --json plan.json --export-md > summary.md

# Export JSON for CI/CD
tf-plan-tui --json plan.json --export-json > plan-data.json
```

Example markdown output:

```markdown
## 📊 Terraform Plan Summary

**Risk Score:** ⚠️  **HIGH** 72/100

| Action | Count |
|--------|-------|
| 🟢 Create | 12 |
| 🟡 Update | 18 |
| 🔴 Delete | 5 |
| ⚠️  Replace | 8 |
| **Total** | **43** |

### ⚠️  Risk Factors
- Database deletion/replacement: module.db.aws_db_instance.main
- Network infrastructure change: module.vpc.aws_vpc.main
- Security-relevant change in aws_security_group.web: ingress.0.cidr_blocks
```

#### Risk-Only Mode (CI/CD)

```bash
# Check risk score (useful in CI pipelines)
tf-plan-tui --json plan.json --risk-only

# Output:
# Risk Score: 72/100 (high)
#
# Risk Factors:
#   - Database deletion/replacement: module.db.aws_db_instance.main
#   - 5 resource(s) will be deleted

# Use in CI
if tf-plan-tui --json plan.json --risk-only | grep -q "critical"; then
  echo "Critical risk detected, manual approval required"
  exit 1
fi
```

## Risk Scoring

The tool automatically calculates a risk score (0-100) based on:

- **Destructive operations**: Deletes (10 pts each), Replaces (15 pts each)
- **Critical resources**: Databases (+20), Networks (+15), IAM (+10), Storage (+10)
- **Security changes**: CIDR to 0.0.0.0/0, publicly_accessible=true, encryption disabled
- **Plan volume**: Large plans (>20 changes) add risk points

| Score | Level | Indicator |
|-------|-------|-----------|
| 0-39 | Low | ✓ |
| 40-69 | Medium | ⚡ |
| 70-89 | High | ⚠️  |
| 90-100 | Critical | ⛔ |

## Security Highlighting

The tool highlights potentially dangerous configuration changes:

- ⚠️  **Network**: CIDR blocks opening to 0.0.0.0/0
- ⚠️  **Encryption**: Encrypted resources becoming unencrypted
- ⚠️  **Access**: publicly_accessible changing from false to true
- ⚠️  **Protection**: deletion_protection disabled
- ⚠️  **IAM**: Wildcard actions in policies

These are highlighted in **red** in the detail pane with a "SECURITY" tag.

## Examples

### Example 1: Daily Workflow

```bash
# 1. Make infrastructure changes
vim main.tf

# 2. Preview in TUI
tf-plan-tui

# 3. Filter to just creates and updates
# (Press 'f' to cycle filters)

# 4. Check risk score
# (Press 'r')

# 5. Export for PR
# (Press 'e')

# 6. Apply changes
terraform apply tfplan
```

### Example 2: CI/CD Integration

```yaml
# .github/workflows/terraform.yml
- name: Terraform Plan
  run: terraform plan -out=tfplan

- name: Generate Plan Summary
  run: |
    terraform show -json tfplan > plan.json
    tf-plan-tui --json plan.json --export-md > plan-summary.md

- name: Comment on PR
  uses: actions/github-script@v6
  with:
    script: |
      const fs = require('fs');
      const summary = fs.readFileSync('plan-summary.md', 'utf8');
      github.rest.issues.createComment({
        issue_number: context.issue.number,
        owner: context.repo.owner,
        repo: context.repo.repo,
        body: summary
      });

- name: Check Risk Score
  run: |
    tf-plan-tui --json plan.json --risk-only
    RISK=$(tf-plan-tui --json plan.json --risk-only | grep "Risk Score" | awk '{print $3}' | cut -d'/' -f1)
    if [ $RISK -gt 70 ]; then
      echo "::warning::High risk score: $RISK"
    fi
```

### Example 3: Large Infrastructure Changes

```bash
# Working with a 200+ resource plan
tf-plan-tui --json large-plan.json

# Toggle module grouping (press 'm') to organize by module
# Search for specific resources (press '/')
# Scroll detail pane independently (Tab to switch, then j/k to scroll)
```

## Configuration

### Environment Variables

- `NO_COLOR=1` — Disable colors (same as --no-color flag)

## Development

### Prerequisites

- Go 1.21+
- Terraform (for testing auto-run mode)

### Build

```bash
make build        # Build binary
make test         # Run tests
make lint         # Run linter
make run          # Run with test data
make install      # Install to $GOPATH/bin
```

### Tests

```bash
make test                # All tests with race detector
make test-short          # Quick tests
make coverage            # Generate coverage report
```

### Project Structure

```
tf-plan-tui/
├── cmd/                # CLI entry point
│   └── root.go
├── internal/
│   ├── parser/         # Terraform JSON parsing
│   ├── tui/            # Bubble Tea UI
│   └── plan/           # Risk analysis & export
├── testdata/           # Test fixtures
└── Makefile
```

## Roadmap

- [ ] Mouse support
- [ ] Copy to clipboard (y key)
- [ ] Diff between two plans
- [ ] Watch mode (live updates)
- [ ] Custom risk rules via config file
- [ ] Remote state browsing

## Contributing

Contributions welcome! Please open an issue or PR.

## License

Apache 2.0

## Credits

Built with:
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) — Styling
- [Bubbles](https://github.com/charmbracelet/bubbles) — Components
- [Cobra](https://github.com/spf13/cobra) — CLI

Inspired by [k9s](https://k9scli.io/), [lazydocker](https://github.com/jesseduffield/lazydocker), and [lazygit](https://github.com/jesseduffield/lazygit).

---

🤖 Made with Claude Code
