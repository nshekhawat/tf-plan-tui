package parser

import (
	"os"
	"testing"
)

func TestParsePlan_Simple(t *testing.T) {
	f, err := os.Open("../../testdata/simple_plan.json")
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()

	plan, err := Parse(f)
	if err != nil {
		t.Fatalf("parsing plan: %v", err)
	}

	if plan.TerraformVersion != "1.7.0" {
		t.Errorf("terraform version = %q, want %q", plan.TerraformVersion, "1.7.0")
	}
	if plan.FormatVersion != "1.2" {
		t.Errorf("format version = %q, want %q", plan.FormatVersion, "1.2")
	}

	if len(plan.Resources) != 5 {
		t.Fatalf("got %d resources, want 5", len(plan.Resources))
	}

	if plan.Stats.Creates != 2 {
		t.Errorf("creates = %d, want 2", plan.Stats.Creates)
	}
	if plan.Stats.Updates != 2 {
		t.Errorf("updates = %d, want 2", plan.Stats.Updates)
	}
	if plan.Stats.Deletes != 1 {
		t.Errorf("deletes = %d, want 1", plan.Stats.Deletes)
	}
	if plan.Stats.TotalChanges != 5 {
		t.Errorf("total changes = %d, want 5", plan.Stats.TotalChanges)
	}

	// Check first resource
	r := plan.Resources[0]
	if r.Address != "aws_s3_bucket.logs" {
		t.Errorf("first resource address = %q, want %q", r.Address, "aws_s3_bucket.logs")
	}
	if r.Action != ActionCreate {
		t.Errorf("first resource action = %q, want %q", r.Action, ActionCreate)
	}
	if r.ResourceType != "aws_s3_bucket" {
		t.Errorf("first resource type = %q, want %q", r.ResourceType, "aws_s3_bucket")
	}

	// Check outputs
	if len(plan.Outputs) != 1 {
		t.Fatalf("got %d outputs, want 1", len(plan.Outputs))
	}
	if plan.Outputs[0].Name != "bucket_arn" {
		t.Errorf("output name = %q, want %q", plan.Outputs[0].Name, "bucket_arn")
	}
}

func TestParsePlan_NoChanges(t *testing.T) {
	f, err := os.Open("../../testdata/no_changes_plan.json")
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()

	plan, err := Parse(f)
	if err != nil {
		t.Fatalf("parsing plan: %v", err)
	}

	if len(plan.Resources) != 0 {
		t.Errorf("got %d resources, want 0", len(plan.Resources))
	}
	if plan.Stats.TotalChanges != 0 {
		t.Errorf("total changes = %d, want 0", plan.Stats.TotalChanges)
	}
}

func TestParsePlan_Destructive(t *testing.T) {
	f, err := os.Open("../../testdata/destructive_plan.json")
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()

	plan, err := Parse(f)
	if err != nil {
		t.Fatalf("parsing plan: %v", err)
	}

	if plan.Stats.Deletes != 5 {
		t.Errorf("deletes = %d, want 5", plan.Stats.Deletes)
	}
	if plan.Stats.Replaces != 2 {
		t.Errorf("replaces = %d, want 2", plan.Stats.Replaces)
	}
	if plan.Stats.TotalChanges != 7 {
		t.Errorf("total changes = %d, want 7", plan.Stats.TotalChanges)
	}

	// Check that replace is correctly identified
	vpc := plan.Resources[0]
	if vpc.Action != ActionReplace {
		t.Errorf("vpc action = %q, want %q", vpc.Action, ActionReplace)
	}
	if vpc.ActionReason != "replace_because_cannot_update" {
		t.Errorf("vpc action_reason = %q, want %q", vpc.ActionReason, "replace_because_cannot_update")
	}
}

func TestParsePlan_Modules(t *testing.T) {
	f, err := os.Open("../../testdata/module_plan.json")
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()

	plan, err := Parse(f)
	if err != nil {
		t.Fatalf("parsing plan: %v", err)
	}

	if len(plan.Resources) != 7 {
		t.Fatalf("got %d resources, want 7", len(plan.Resources))
	}

	// Check nested module address
	subnet := plan.Resources[1]
	if subnet.ModuleAddress != "module.vpc.module.subnets" {
		t.Errorf("subnet module address = %q, want %q", subnet.ModuleAddress, "module.vpc.module.subnets")
	}
	if subnet.Index != float64(0) {
		t.Errorf("subnet index = %v (%T), want 0", subnet.Index, subnet.Index)
	}
}

func TestParsePlan_Sensitive(t *testing.T) {
	f, err := os.Open("../../testdata/destructive_plan.json")
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()

	plan, err := Parse(f)
	if err != nil {
		t.Fatalf("parsing plan: %v", err)
	}

	// The db_instance has a sensitive password field
	var db *ResourceChange
	for i := range plan.Resources {
		if plan.Resources[i].ResourceType == "aws_db_instance" {
			db = &plan.Resources[i]
			break
		}
	}
	if db == nil {
		t.Fatal("db_instance not found")
	}

	if !db.Sensitive["password"] {
		t.Error("password should be marked as sensitive")
	}

	// Check that AttrChanges marks the password as sensitive
	for _, attr := range db.AttrChanges {
		if attr.Path == "password" && !attr.IsSensitive {
			t.Error("password attr change should be marked sensitive")
		}
	}
}

func TestParsePlan_AttrChanges(t *testing.T) {
	f, err := os.Open("../../testdata/simple_plan.json")
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()

	plan, err := Parse(f)
	if err != nil {
		t.Fatalf("parsing plan: %v", err)
	}

	// The security group update should have attr changes
	var sg *ResourceChange
	for i := range plan.Resources {
		if plan.Resources[i].ResourceType == "aws_security_group" {
			sg = &plan.Resources[i]
			break
		}
	}
	if sg == nil {
		t.Fatal("security_group not found")
	}

	if len(sg.AttrChanges) == 0 {
		t.Fatal("expected attribute changes for security group update")
	}

	// Check that tags.Environment changed
	found := false
	for _, attr := range sg.AttrChanges {
		if attr.Path == "tags.Environment" {
			found = true
			if attr.Before != "staging" {
				t.Errorf("tags.Environment before = %q, want %q", attr.Before, "staging")
			}
			if attr.After != "production" {
				t.Errorf("tags.Environment after = %q, want %q", attr.After, "production")
			}
			if attr.Action != "changed" {
				t.Errorf("tags.Environment action = %q, want %q", attr.Action, "changed")
			}
		}
	}
	if !found {
		t.Error("tags.Environment change not found in attr changes")
	}
}

func TestActionIcon(t *testing.T) {
	tests := []struct {
		action Action
		want   string
	}{
		{ActionCreate, "+"},
		{ActionUpdate, "~"},
		{ActionDelete, "-"},
		{ActionReplace, "±"},
		{ActionRead, ">"},
		{ActionNoOp, " "},
	}
	for _, tt := range tests {
		got := ActionIcon(tt.action)
		if got != tt.want {
			t.Errorf("ActionIcon(%q) = %q, want %q", tt.action, got, tt.want)
		}
	}
}

func TestShortAddress(t *testing.T) {
	tests := []struct {
		rc   ResourceChange
		want string
	}{
		{
			rc:   ResourceChange{ResourceType: "aws_s3_bucket", ResourceName: "logs"},
			want: "aws_s3_bucket.logs",
		},
		{
			rc:   ResourceChange{ModuleAddress: "module.vpc", ResourceType: "aws_subnet", ResourceName: "private", Index: float64(0)},
			want: "module.vpc.aws_subnet.private[0]",
		},
		{
			rc:   ResourceChange{ModuleAddress: "module.vpc.module.subnets", ResourceType: "aws_subnet", ResourceName: "this", Index: "az-a"},
			want: "module.vpc.module.subnets.aws_subnet.this[az-a]",
		},
	}
	for _, tt := range tests {
		got := ShortAddress(tt.rc)
		if got != tt.want {
			t.Errorf("ShortAddress() = %q, want %q", got, tt.want)
		}
	}
}
