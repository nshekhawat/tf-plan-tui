package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hashicorp/tf-plan-tui/internal/parser"
	"github.com/hashicorp/tf-plan-tui/internal/plan"
	"github.com/hashicorp/tf-plan-tui/internal/tui"
	"github.com/spf13/cobra"
)

var (
	jsonFile   string
	planFile   string
	exportMd   bool
	exportJSON bool
	riskOnly   bool
	filter     string
	dir        string
	noColor    bool
)

var rootCmd = &cobra.Command{
	Use:   "tf-plan-tui [file | -]",
	Short: "Interactive TUI for Terraform plan output",
	Long:  "A terminal UI that transforms terraform plan JSON into a navigable, filterable interface.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  run,
}

func init() {
	rootCmd.Flags().StringVarP(&jsonFile, "json", "j", "", "Path to terraform plan JSON file")
	rootCmd.Flags().StringVarP(&planFile, "plan", "p", "", "Path to terraform plan binary file")
	rootCmd.Flags().BoolVar(&exportMd, "export-md", false, "Export plan summary as markdown and exit")
	rootCmd.Flags().BoolVar(&exportJSON, "export-json", false, "Export plan summary as JSON and exit")
	rootCmd.Flags().BoolVar(&riskOnly, "risk-only", false, "Print risk score and exit (for CI/CD)")
	rootCmd.Flags().StringVar(&filter, "filter", "", "Pre-filter to action: create, update, delete, replace")
	rootCmd.Flags().StringVar(&dir, "dir", "", "Run terraform plan in specific directory")
	rootCmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colors (for piping)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	var r io.Reader
	var cleanup func()

	// Determine input source
	switch {
	case len(args) == 1 && args[0] == "-":
		r = os.Stdin
	case jsonFile != "":
		f, err := os.Open(jsonFile)
		if err != nil {
			return fmt.Errorf("opening JSON file: %w", err)
		}
		defer f.Close()
		r = f
	case planFile != "":
		// Convert terraform plan file to JSON
		jsonData, err := convertPlanToJSON(planFile)
		if err != nil {
			return fmt.Errorf("converting plan to JSON: %w", err)
		}
		r = jsonData
	case len(args) == 1:
		// Check if it's a JSON file or binary plan
		ext := filepath.Ext(args[0])
		if ext == ".json" {
			f, err := os.Open(args[0])
			if err != nil {
				return fmt.Errorf("opening file: %w", err)
			}
			defer f.Close()
			r = f
		} else {
			// Treat as binary plan
			jsonData, err := convertPlanToJSON(args[0])
			if err != nil {
				return fmt.Errorf("converting plan to JSON: %w", err)
			}
			r = jsonData
		}
	default:
		// Auto-run mode: check for existing tfplan, otherwise run terraform plan
		workDir := dir
		if workDir == "" {
			workDir = "."
		}

		// Check if tfplan exists
		tfplanPath := filepath.Join(workDir, "tfplan")
		if _, err := os.Stat(tfplanPath); err == nil {
			jsonData, err := convertPlanToJSON(tfplanPath)
			if err != nil {
				return fmt.Errorf("converting existing tfplan: %w", err)
			}
			r = jsonData
		} else {
			// Run terraform plan
			fmt.Fprintf(os.Stderr, "Running terraform plan in %s...\n", workDir)
			tempPlan := filepath.Join(workDir, ".tf-plan-tui.tfplan")
			jsonData, err := runTerraformPlan(workDir, tempPlan)
			if err != nil {
				return fmt.Errorf("running terraform plan: %w", err)
			}
			r = jsonData
			cleanup = func() {
				os.Remove(tempPlan)
			}
		}
	}

	if cleanup != nil {
		defer cleanup()
	}

	// Parse plan
	planData, err := parser.Parse(r)
	if err != nil {
		return fmt.Errorf("parsing plan: %w", err)
	}

	// Calculate risk
	riskAnalysis := plan.CalculateRiskScore(planData)

	// Handle non-interactive modes
	if riskOnly {
		fmt.Printf("Risk Score: %d/100 (%s)\n", riskAnalysis.Score, riskAnalysis.Level)
		if len(riskAnalysis.Reasons) > 0 {
			fmt.Println("\nRisk Factors:")
			for _, reason := range riskAnalysis.Reasons {
				fmt.Printf("  - %s\n", reason)
			}
		}
		return nil
	}

	if exportMd {
		markdown := plan.ExportMarkdown(planData, riskAnalysis)
		fmt.Print(markdown)
		return nil
	}

	if exportJSON {
		json := plan.ExportJSON(planData, riskAnalysis)
		fmt.Print(json)
		return nil
	}

	// Interactive TUI mode
	if noColor {
		// TODO: Set environment variable for lipgloss to disable colors
		os.Setenv("NO_COLOR", "1")
	}

	m := tui.NewModel(planData)

	// Apply pre-filter if specified
	if filter != "" {
		var action parser.Action
		switch filter {
		case "create":
			action = parser.ActionCreate
		case "update":
			action = parser.ActionUpdate
		case "delete":
			action = parser.ActionDelete
		case "replace":
			action = parser.ActionReplace
		default:
			return fmt.Errorf("invalid filter: %s (must be create, update, delete, or replace)", filter)
		}
		// TODO: Set initial filter in model
		_ = action
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}
	return nil
}

func convertPlanToJSON(planPath string) (io.Reader, error) {
	cmd := exec.Command("terraform", "show", "-json", planPath)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("terraform show failed: %s", string(exitErr.Stderr))
		}
		return nil, err
	}
	return &bytesReader{data: output}, nil
}

func runTerraformPlan(workDir, outputPath string) (io.Reader, error) {
	// Run terraform plan -out
	cmd := exec.Command("terraform", "plan", "-out="+outputPath)
	cmd.Dir = workDir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("terraform plan failed: %w", err)
	}

	// Convert to JSON
	return convertPlanToJSON(outputPath)
}

// bytesReader wraps []byte to implement io.Reader
type bytesReader struct {
	data   []byte
	offset int
}

func (br *bytesReader) Read(p []byte) (n int, err error) {
	if br.offset >= len(br.data) {
		return 0, io.EOF
	}
	n = copy(p, br.data[br.offset:])
	br.offset += n
	return n, nil
}
