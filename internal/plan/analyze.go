package plan

import (
	"fmt"
	"strings"

	"github.com/hashicorp/tf-plan-tui/internal/parser"
)

// RiskAnalysis contains risk score and reasons.
type RiskAnalysis struct {
	Score   int
	Level   string
	Reasons []string
}

// CalculateRiskScore computes a risk score (0-100) based on heuristics.
func CalculateRiskScore(plan *parser.PlanSummary) RiskAnalysis {
	score := 0
	var reasons []string

	// Destructive operations (high weight)
	score += plan.Stats.Deletes * 10     // Each delete = 10 points
	score += plan.Stats.Replaces * 15    // Each replace = 15 points (more risky)

	if plan.Stats.Deletes > 0 {
		reasons = append(reasons, fmt.Sprintf("%d resource(s) will be deleted", plan.Stats.Deletes))
	}
	if plan.Stats.Replaces > 0 {
		reasons = append(reasons, fmt.Sprintf("%d resource(s) will be replaced (destroy+create)", plan.Stats.Replaces))
	}

	// Volume of changes
	if plan.Stats.TotalChanges > 20 {
		score += 10
		reasons = append(reasons, fmt.Sprintf("Large plan: %d total changes", plan.Stats.TotalChanges))
	}
	if plan.Stats.TotalChanges > 50 {
		score += 15
		reasons = append(reasons, fmt.Sprintf("Very large plan: %d total changes", plan.Stats.TotalChanges))
	}

	// Critical resource types
	for _, r := range plan.Resources {
		if r.Action == parser.ActionDelete || r.Action == parser.ActionReplace {
			switch r.ResourceType {
			case "aws_db_instance", "aws_rds_cluster", "azurerm_sql_database", "google_sql_database_instance":
				score += 20
				reasons = append(reasons, fmt.Sprintf("Database deletion/replacement: %s", r.Address))
			case "aws_vpc", "aws_subnet", "azurerm_virtual_network", "google_compute_network":
				score += 15
				reasons = append(reasons, fmt.Sprintf("Network infrastructure change: %s", r.Address))
			case "aws_eks_cluster", "aws_ecs_cluster", "azurerm_kubernetes_cluster", "google_container_cluster":
				score += 20
				reasons = append(reasons, fmt.Sprintf("Container cluster change: %s", r.Address))
			case "aws_iam_role", "aws_iam_policy", "azurerm_role_assignment", "google_project_iam_member":
				score += 10
				reasons = append(reasons, fmt.Sprintf("IAM/permissions change: %s", r.Address))
			case "aws_s3_bucket", "azurerm_storage_account", "google_storage_bucket":
				score += 10
				reasons = append(reasons, fmt.Sprintf("Storage deletion: %s", r.Address))
			case "aws_dynamodb_table":
				score += 15
				reasons = append(reasons, fmt.Sprintf("DynamoDB table deletion: %s", r.Address))
			}
		}

		// Security-relevant attribute changes
		for _, attr := range r.AttrChanges {
			if IsSecurityRelevantChange(r, attr) {
				score += 5
				reasons = append(reasons, fmt.Sprintf("Security-relevant change in %s: %s", r.Address, attr.Path))
			}
		}
	}

	// Cap at 100
	if score > 100 {
		score = 100
	}

	return RiskAnalysis{
		Score:   score,
		Level:   riskLevel(score),
		Reasons: reasons,
	}
}

func riskLevel(score int) string {
	switch {
	case score >= 90:
		return "critical"
	case score >= 70:
		return "high"
	case score >= 40:
		return "medium"
	default:
		return "low"
	}
}

// IsSecurityRelevantChange checks if an attribute change has security implications.
func IsSecurityRelevantChange(r parser.ResourceChange, attr parser.AttributeChange) bool {
	path := strings.ToLower(attr.Path)
	before := strings.ToLower(attr.Before)
	after := strings.ToLower(attr.After)

	// CIDR blocks changing to 0.0.0.0/0
	if strings.Contains(path, "cidr_block") && strings.Contains(after, "0.0.0.0/0") {
		return true
	}

	// publicly_accessible changing to true
	if strings.Contains(path, "publicly_accessible") && after == "true" && before == "false" {
		return true
	}

	// encrypted changing to false
	if strings.Contains(path, "encrypt") && after == "false" && before == "true" {
		return true
	}

	// deletion_protection changing to false
	if strings.Contains(path, "deletion_protection") && after == "false" && before == "true" {
		return true
	}

	// IAM policy with wildcard actions
	if strings.Contains(path, "policy") && strings.Contains(after, "\"Action\":[\"*\"]") {
		return true
	}
	if strings.Contains(path, "policy") && strings.Contains(after, "\"action\":\"*\"") {
		return true
	}

	// Security group ingress from anywhere
	if r.ResourceType == "aws_security_group" && strings.Contains(path, "ingress") && strings.Contains(after, "0.0.0.0/0") {
		return true
	}

	// S3 bucket ACL changing to public
	if r.ResourceType == "aws_s3_bucket" && strings.Contains(path, "acl") && (strings.Contains(after, "public") || strings.Contains(after, "authenticated-read")) {
		return true
	}

	return false
}

// RiskIcon returns an emoji/symbol representing the risk level.
func RiskIcon(level string) string {
	switch level {
	case "critical":
		return "⛔"
	case "high":
		return "⚠️ "
	case "medium":
		return "⚡"
	case "low":
		return "✓"
	default:
		return "?"
	}
}

// RiskColor returns a color name for the risk level.
func RiskColor(level string) string {
	switch level {
	case "critical":
		return "red"
	case "high":
		return "orange"
	case "medium":
		return "yellow"
	case "low":
		return "green"
	default:
		return "gray"
	}
}
