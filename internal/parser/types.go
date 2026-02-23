package parser

// PlanSummary is the top-level parsed plan.
type PlanSummary struct {
	TerraformVersion string
	FormatVersion    string
	Timestamp        string
	RootModule       string
	Resources        []ResourceChange
	Outputs          []OutputChange
	Stats            PlanStats
}

// PlanStats holds aggregate counts by action type.
type PlanStats struct {
	TotalChanges int
	Creates      int
	Updates      int
	Deletes      int
	Replaces     int
	NoOps        int
	Reads        int
}

// ResourceChange represents a single resource change in the plan.
type ResourceChange struct {
	Address       string
	ModuleAddress string
	ResourceType  string
	ResourceName  string
	ProviderName  string
	Action        Action
	ActionReason  string
	Before        map[string]any
	After         map[string]any
	AfterUnknown  map[string]any
	Sensitive     map[string]bool
	AttrChanges   []AttributeChange
	Index         any
}

// AttributeChange represents a single attribute diff.
type AttributeChange struct {
	Path        string
	Before      string
	After       string
	Action      string // "changed", "added", "removed"
	IsSensitive bool
}

// OutputChange represents a change to a Terraform output.
type OutputChange struct {
	Name   string
	Action Action
	Before any
	After  any
}

// Action represents the type of change for a resource.
type Action string

const (
	ActionCreate  Action = "create"
	ActionUpdate  Action = "update"
	ActionDelete  Action = "delete"
	ActionReplace Action = "replace"
	ActionRead    Action = "read"
	ActionNoOp    Action = "no-op"
)
