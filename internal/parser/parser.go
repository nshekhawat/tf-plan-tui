package parser

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// rawPlan mirrors the terraform show -json output structure.
type rawPlan struct {
	FormatVersion    string                       `json:"format_version"`
	TerraformVersion string                       `json:"terraform_version"`
	ResourceChanges  []rawResourceChange          `json:"resource_changes"`
	OutputChanges    map[string]rawOutputChange    `json:"output_changes"`
}

type rawResourceChange struct {
	Address       string    `json:"address"`
	ModuleAddress string    `json:"module_address"`
	Mode          string    `json:"mode"`
	Type          string    `json:"type"`
	Name          string    `json:"name"`
	Index         any       `json:"index"`
	ProviderName  string    `json:"provider_name"`
	Change        rawChange `json:"change"`
	ActionReason  string    `json:"action_reason"`
}

type rawChange struct {
	Actions         []string       `json:"actions"`
	Before          map[string]any `json:"before"`
	After           map[string]any `json:"after"`
	AfterUnknown    map[string]any `json:"after_unknown"`
	BeforeSensitive any            `json:"before_sensitive"`
	AfterSensitive  any            `json:"after_sensitive"`
}

type rawOutputChange struct {
	Actions         []string `json:"actions"`
	Before          any      `json:"before"`
	After           any      `json:"after"`
	BeforeSensitive bool     `json:"before_sensitive"`
	AfterSensitive  bool     `json:"after_sensitive"`
}

// Parse reads terraform plan JSON from r and returns a PlanSummary.
func Parse(r io.Reader) (*PlanSummary, error) {
	var raw rawPlan
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding plan JSON: %w", err)
	}

	plan := &PlanSummary{
		TerraformVersion: raw.TerraformVersion,
		FormatVersion:    raw.FormatVersion,
	}

	for _, rc := range raw.ResourceChanges {
		action := parseAction(rc.Change.Actions)

		sensitive := mergeSensitive(rc.Change.BeforeSensitive, rc.Change.AfterSensitive)
		attrChanges := computeAttrChanges(rc.Change.Before, rc.Change.After, sensitive)

		res := ResourceChange{
			Address:       rc.Address,
			ModuleAddress: rc.ModuleAddress,
			ResourceType:  rc.Type,
			ResourceName:  rc.Name,
			ProviderName:  rc.ProviderName,
			Action:        action,
			ActionReason:  rc.ActionReason,
			Before:        rc.Change.Before,
			After:         rc.Change.After,
			AfterUnknown:  rc.Change.AfterUnknown,
			Sensitive:     sensitive,
			AttrChanges:   attrChanges,
			Index:         rc.Index,
		}
		plan.Resources = append(plan.Resources, res)

		switch action {
		case ActionCreate:
			plan.Stats.Creates++
			plan.Stats.TotalChanges++
		case ActionUpdate:
			plan.Stats.Updates++
			plan.Stats.TotalChanges++
		case ActionDelete:
			plan.Stats.Deletes++
			plan.Stats.TotalChanges++
		case ActionReplace:
			plan.Stats.Replaces++
			plan.Stats.TotalChanges++
		case ActionRead:
			plan.Stats.Reads++
		case ActionNoOp:
			plan.Stats.NoOps++
		}
	}

	for name, oc := range raw.OutputChanges {
		action := parseAction(oc.Actions)
		plan.Outputs = append(plan.Outputs, OutputChange{
			Name:   name,
			Action: action,
			Before: oc.Before,
			After:  oc.After,
		})
	}

	sort.Slice(plan.Outputs, func(i, j int) bool {
		return plan.Outputs[i].Name < plan.Outputs[j].Name
	})

	return plan, nil
}

func parseAction(actions []string) Action {
	if len(actions) == 0 {
		return ActionNoOp
	}
	if len(actions) == 1 {
		switch actions[0] {
		case "create":
			return ActionCreate
		case "delete":
			return ActionDelete
		case "update":
			return ActionUpdate
		case "read":
			return ActionRead
		case "no-op":
			return ActionNoOp
		}
	}
	if len(actions) == 2 {
		// ["delete", "create"] or ["create", "delete"] both mean replace
		a, b := actions[0], actions[1]
		if (a == "delete" && b == "create") || (a == "create" && b == "delete") {
			return ActionReplace
		}
	}
	return ActionNoOp
}

// mergeSensitive combines before_sensitive and after_sensitive into a flat map.
// Terraform represents these as either a bool or a map[string]any.
func mergeSensitive(before, after any) map[string]bool {
	result := make(map[string]bool)
	collectSensitive(result, "", before)
	collectSensitive(result, "", after)
	return result
}

func collectSensitive(result map[string]bool, prefix string, v any) {
	switch val := v.(type) {
	case bool:
		if val && prefix != "" {
			result[prefix] = true
		}
	case map[string]any:
		for k, child := range val {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			collectSensitive(result, path, child)
		}
	}
}

// computeAttrChanges produces a flat list of attribute diffs between before and after.
func computeAttrChanges(before, after map[string]any, sensitive map[string]bool) []AttributeChange {
	beforeFlat := flattenMap("", before)
	afterFlat := flattenMap("", after)

	allKeys := make(map[string]struct{})
	for k := range beforeFlat {
		allKeys[k] = struct{}{}
	}
	for k := range afterFlat {
		allKeys[k] = struct{}{}
	}

	sortedKeys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	var changes []AttributeChange
	for _, key := range sortedKeys {
		bv, bOk := beforeFlat[key]
		av, aOk := afterFlat[key]

		isSensitive := sensitive[key]

		switch {
		case bOk && aOk && bv != av:
			changes = append(changes, AttributeChange{
				Path:        key,
				Before:      bv,
				After:       av,
				Action:      "changed",
				IsSensitive: isSensitive,
			})
		case !bOk && aOk:
			changes = append(changes, AttributeChange{
				Path:        key,
				Before:      "",
				After:       av,
				Action:      "added",
				IsSensitive: isSensitive,
			})
		case bOk && !aOk:
			changes = append(changes, AttributeChange{
				Path:        key,
				Before:      bv,
				After:       "",
				Action:      "removed",
				IsSensitive: isSensitive,
			})
		}
	}
	return changes
}

// flattenMap flattens a nested map into dot-notation keys with string values.
func flattenMap(prefix string, m map[string]any) map[string]string {
	result := make(map[string]string)
	for k, v := range m {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]any:
			for fk, fv := range flattenMap(key, val) {
				result[fk] = fv
			}
		case []any:
			for fk, fv := range flattenSlice(key, val) {
				result[fk] = fv
			}
		case nil:
			result[key] = "<null>"
		default:
			result[key] = fmt.Sprintf("%v", val)
		}
	}
	return result
}

func flattenSlice(prefix string, s []any) map[string]string {
	result := make(map[string]string)
	for i, v := range s {
		key := fmt.Sprintf("%s.%d", prefix, i)
		switch val := v.(type) {
		case map[string]any:
			for fk, fv := range flattenMap(key, val) {
				result[fk] = fv
			}
		case []any:
			for fk, fv := range flattenSlice(key, val) {
				result[fk] = fv
			}
		case nil:
			result[key] = "<null>"
		default:
			result[key] = fmt.Sprintf("%v", val)
		}
	}
	// Also store the length for comparison purposes
	result[prefix+".#"] = fmt.Sprintf("%d", len(s))
	return result
}

// ActionIcon returns a display icon for the given action.
func ActionIcon(a Action) string {
	switch a {
	case ActionCreate:
		return "+"
	case ActionUpdate:
		return "~"
	case ActionDelete:
		return "-"
	case ActionReplace:
		return "±"
	case ActionRead:
		return ">"
	case ActionNoOp:
		return " "
	default:
		return "?"
	}
}

// ShortAddress returns a shortened version of a resource address,
// trimming the module prefix for display.
func ShortAddress(rc ResourceChange) string {
	var b strings.Builder
	if rc.ModuleAddress != "" {
		b.WriteString(rc.ModuleAddress)
		b.WriteString(".")
	}
	b.WriteString(rc.ResourceType)
	b.WriteString(".")
	b.WriteString(rc.ResourceName)
	if rc.Index != nil {
		b.WriteString(fmt.Sprintf("[%v]", rc.Index))
	}
	return b.String()
}
