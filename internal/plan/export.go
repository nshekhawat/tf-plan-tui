package plan

import (
	"fmt"
	"strings"

	"github.com/hashicorp/tf-plan-tui/internal/parser"
)

// ExportMarkdown exports the plan summary as markdown suitable for PR comments.
func ExportMarkdown(plan *parser.PlanSummary, risk RiskAnalysis) string {
	var b strings.Builder

	b.WriteString("## 📊 Terraform Plan Summary\n\n")

	// Risk score
	icon := RiskIcon(risk.Level)
	b.WriteString(fmt.Sprintf("**Risk Score:** %s **%s** %d/100\n\n", icon, strings.ToUpper(risk.Level), risk.Score))

	// Stats table
	s := plan.Stats
	b.WriteString("| Action | Count |\n")
	b.WriteString("|--------|-------|\n")
	b.WriteString(fmt.Sprintf("| 🟢 Create | %d |\n", s.Creates))
	b.WriteString(fmt.Sprintf("| 🟡 Update | %d |\n", s.Updates))
	b.WriteString(fmt.Sprintf("| 🔴 Delete | %d |\n", s.Deletes))
	b.WriteString(fmt.Sprintf("| ⚠️  Replace | %d |\n", s.Replaces))
	b.WriteString(fmt.Sprintf("| **Total** | **%d** |\n", s.TotalChanges))
	b.WriteString("\n")

	// Risk factors
	if len(risk.Reasons) > 0 {
		b.WriteString(fmt.Sprintf("### %s Risk Factors\n\n", icon))
		for _, reason := range risk.Reasons {
			b.WriteString(fmt.Sprintf("- %s\n", reason))
		}
		b.WriteString("\n")
	}

	// Destructive changes
	deletes := []parser.ResourceChange{}
	replaces := []parser.ResourceChange{}
	for _, r := range plan.Resources {
		if r.Action == parser.ActionDelete {
			deletes = append(deletes, r)
		} else if r.Action == parser.ActionReplace {
			replaces = append(replaces, r)
		}
	}

	if len(deletes) > 0 || len(replaces) > 0 {
		b.WriteString("### 🔴 Destructive Changes\n\n")
		b.WriteString("| Resource | Action | Reason |\n")
		b.WriteString("|----------|--------|--------|\n")

		for _, r := range replaces {
			reason := r.ActionReason
			if reason == "" {
				reason = "-"
			}
			b.WriteString(fmt.Sprintf("| `%s` | replace | %s |\n", r.Address, reason))
		}
		for _, r := range deletes {
			b.WriteString(fmt.Sprintf("| `%s` | delete | - |\n", r.Address))
		}
		b.WriteString("\n")
	}

	// New resources
	creates := []parser.ResourceChange{}
	for _, r := range plan.Resources {
		if r.Action == parser.ActionCreate {
			creates = append(creates, r)
		}
	}

	if len(creates) > 0 {
		b.WriteString("### 🟢 New Resources\n\n")
		if len(creates) > 10 {
			b.WriteString(fmt.Sprintf("<details><summary>%d resources (click to expand)</summary>\n\n", len(creates)))
		}
		b.WriteString("| Resource | Type |\n")
		b.WriteString("|----------|------|\n")
		for _, r := range creates {
			b.WriteString(fmt.Sprintf("| `%s` | %s |\n", r.Address, r.ResourceType))
		}
		if len(creates) > 10 {
			b.WriteString("\n</details>\n")
		}
		b.WriteString("\n")
	}

	// Updates
	updates := []parser.ResourceChange{}
	for _, r := range plan.Resources {
		if r.Action == parser.ActionUpdate {
			updates = append(updates, r)
		}
	}

	if len(updates) > 0 {
		b.WriteString("### 🟡 Updated Resources\n\n")
		if len(updates) > 10 {
			b.WriteString(fmt.Sprintf("<details><summary>%d resources (click to expand)</summary>\n\n", len(updates)))
		}
		b.WriteString("| Resource | Changes |\n")
		b.WriteString("|----------|----------|\n")
		for _, r := range updates {
			changeCount := len(r.AttrChanges)
			b.WriteString(fmt.Sprintf("| `%s` | %d attribute(s) |\n", r.Address, changeCount))
		}
		if len(updates) > 10 {
			b.WriteString("\n</details>\n")
		}
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("---\n")
	b.WriteString("🤖 Generated with [tf-plan-tui](https://github.com/hashicorp/tf-plan-tui)\n")

	return b.String()
}

// ExportJSON exports the plan and risk analysis as JSON.
func ExportJSON(plan *parser.PlanSummary, risk RiskAnalysis) string {
	// Simple JSON serialization for now
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(fmt.Sprintf("  \"terraform_version\": \"%s\",\n", plan.TerraformVersion))
	b.WriteString(fmt.Sprintf("  \"total_changes\": %d,\n", plan.Stats.TotalChanges))
	b.WriteString(fmt.Sprintf("  \"creates\": %d,\n", plan.Stats.Creates))
	b.WriteString(fmt.Sprintf("  \"updates\": %d,\n", plan.Stats.Updates))
	b.WriteString(fmt.Sprintf("  \"deletes\": %d,\n", plan.Stats.Deletes))
	b.WriteString(fmt.Sprintf("  \"replaces\": %d,\n", plan.Stats.Replaces))
	b.WriteString("  \"risk\": {\n")
	b.WriteString(fmt.Sprintf("    \"score\": %d,\n", risk.Score))
	b.WriteString(fmt.Sprintf("    \"level\": \"%s\",\n", risk.Level))
	b.WriteString(fmt.Sprintf("    \"reasons\": [\n"))
	for i, r := range risk.Reasons {
		if i > 0 {
			b.WriteString(",\n")
		}
		b.WriteString(fmt.Sprintf("      \"%s\"", escapeJSON(r)))
	}
	b.WriteString("\n    ]\n")
	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.String()
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
