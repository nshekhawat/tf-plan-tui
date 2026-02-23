package plan

import (
	"testing"

	"github.com/hashicorp/tf-plan-tui/internal/parser"
)

func TestCalculateRiskScore_LowRisk(t *testing.T) {
	plan := &parser.PlanSummary{
		Stats: parser.PlanStats{
			Creates:      5,
			Updates:      2,
			TotalChanges: 7,
		},
		Resources: []parser.ResourceChange{
			{ResourceType: "aws_s3_bucket", Action: parser.ActionCreate},
			{ResourceType: "aws_iam_role", Action: parser.ActionCreate},
		},
	}

	result := CalculateRiskScore(plan)
	if result.Score > 20 {
		t.Errorf("Expected low risk score (<= 20), got %d", result.Score)
	}
	if result.Level != "low" {
		t.Errorf("Expected low risk level, got %s", result.Level)
	}
}

func TestCalculateRiskScore_HighRisk_Deletes(t *testing.T) {
	plan := &parser.PlanSummary{
		Stats: parser.PlanStats{
			Deletes:      5,
			TotalChanges: 5,
		},
		Resources: []parser.ResourceChange{
			{ResourceType: "aws_s3_bucket", Action: parser.ActionDelete, Address: "aws_s3_bucket.data"},
			{ResourceType: "aws_iam_role", Action: parser.ActionDelete, Address: "aws_iam_role.admin"},
			{ResourceType: "aws_instance", Action: parser.ActionDelete, Address: "aws_instance.web"},
			{ResourceType: "aws_instance", Action: parser.ActionDelete, Address: "aws_instance.api"},
			{ResourceType: "aws_instance", Action: parser.ActionDelete, Address: "aws_instance.worker"},
		},
	}

	result := CalculateRiskScore(plan)
	// 5 deletes * 10 = 50, plus IAM (10) and S3 (10) = 70
	if result.Score < 50 {
		t.Errorf("Expected high risk score (>= 50), got %d", result.Score)
	}
	if result.Level == "low" {
		t.Errorf("Expected medium/high risk level, got %s", result.Level)
	}
}

func TestCalculateRiskScore_CriticalRisk_Database(t *testing.T) {
	plan := &parser.PlanSummary{
		Stats: parser.PlanStats{
			Replaces:     2,
			TotalChanges: 2,
		},
		Resources: []parser.ResourceChange{
			{
				ResourceType: "aws_db_instance",
				Action:       parser.ActionReplace,
				Address:      "aws_db_instance.main",
			},
			{
				ResourceType: "aws_rds_cluster",
				Action:       parser.ActionReplace,
				Address:      "aws_rds_cluster.prod",
			},
		},
	}

	result := CalculateRiskScore(plan)
	// 2 replaces * 15 = 30, plus 2 db changes * 20 = 40, total 70
	if result.Score < 60 {
		t.Errorf("Expected critical risk score (>= 60), got %d", result.Score)
	}
	if result.Level == "low" {
		t.Errorf("Expected high/critical risk level, got %s", result.Level)
	}
	if len(result.Reasons) < 2 {
		t.Errorf("Expected at least 2 reasons, got %d", len(result.Reasons))
	}
}

func TestCalculateRiskScore_SecurityRelevant(t *testing.T) {
	plan := &parser.PlanSummary{
		Stats: parser.PlanStats{
			Updates:      1,
			TotalChanges: 1,
		},
		Resources: []parser.ResourceChange{
			{
				ResourceType: "aws_security_group",
				Action:       parser.ActionUpdate,
				Address:      "aws_security_group.web",
				AttrChanges: []parser.AttributeChange{
					{
						Path:   "ingress.0.cidr_blocks",
						Before: "[10.0.0.0/8]",
						After:  "[0.0.0.0/0]",
						Action: "changed",
					},
				},
			},
		},
	}

	result := CalculateRiskScore(plan)
	if result.Score < 5 {
		t.Errorf("Expected risk score >= 5 for security change, got %d", result.Score)
	}
	foundSecurityReason := false
	for _, reason := range result.Reasons {
		if contains(reason, "Security-relevant") {
			foundSecurityReason = true
			break
		}
	}
	if !foundSecurityReason {
		t.Errorf("Expected security-relevant reason in results")
	}
}

func TestCalculateRiskScore_LargePlan(t *testing.T) {
	plan := &parser.PlanSummary{
		Stats: parser.PlanStats{
			Creates:      60,
			TotalChanges: 60,
		},
		Resources: make([]parser.ResourceChange, 60),
	}

	for i := range plan.Resources {
		plan.Resources[i] = parser.ResourceChange{
			ResourceType: "aws_instance",
			Action:       parser.ActionCreate,
		}
	}

	result := CalculateRiskScore(plan)
	// Should get points for large plan (> 50 changes)
	if result.Score < 15 {
		t.Errorf("Expected risk score >= 15 for large plan, got %d", result.Score)
	}
}

func TestIsSecurityRelevantChange_CIDRBlock(t *testing.T) {
	r := parser.ResourceChange{ResourceType: "aws_security_group"}
	attr := parser.AttributeChange{
		Path:   "ingress.0.cidr_blocks",
		Before: "[10.0.0.0/8]",
		After:  "[0.0.0.0/0]",
	}

	if !IsSecurityRelevantChange(r, attr) {
		t.Error("Expected CIDR 0.0.0.0/0 change to be security-relevant")
	}
}

func TestIsSecurityRelevantChange_PubliclyAccessible(t *testing.T) {
	r := parser.ResourceChange{ResourceType: "aws_db_instance"}
	attr := parser.AttributeChange{
		Path:   "publicly_accessible",
		Before: "false",
		After:  "true",
	}

	if !IsSecurityRelevantChange(r, attr) {
		t.Error("Expected publicly_accessible=true to be security-relevant")
	}
}

func TestIsSecurityRelevantChange_EncryptionDisabled(t *testing.T) {
	r := parser.ResourceChange{ResourceType: "aws_s3_bucket"}
	attr := parser.AttributeChange{
		Path:   "server_side_encryption_configuration.0.rule.0.apply_server_side_encryption_by_default.0.sse_algorithm",
		Before: "true",
		After:  "false",
	}

	if !IsSecurityRelevantChange(r, attr) {
		t.Error("Expected encryption=false to be security-relevant")
	}
}

func TestRiskLevel(t *testing.T) {
	tests := []struct {
		score int
		want  string
	}{
		{0, "low"},
		{39, "low"},
		{40, "medium"},
		{69, "medium"},
		{70, "high"},
		{89, "high"},
		{90, "critical"},
		{100, "critical"},
	}

	for _, tt := range tests {
		got := riskLevel(tt.score)
		if got != tt.want {
			t.Errorf("riskLevel(%d) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
