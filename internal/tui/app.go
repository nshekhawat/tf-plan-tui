package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hashicorp/tf-plan-tui/internal/parser"
	"github.com/hashicorp/tf-plan-tui/internal/plan"
)

type Pane int

const (
	PaneResourceList Pane = iota
	PaneDetail
)

type displayItem struct {
	isModuleHeader bool
	moduleName     string
	moduleStats    string
	resource       *parser.ResourceChange
	displayIndex   int // Index in filteredRes for resources
}

type Model struct {
	plan         *parser.PlanSummary
	riskAnalysis plan.RiskAnalysis
	filteredRes  []parser.ResourceChange
	displayList  []displayItem // Computed list for rendering (includes module headers)

	activePane   Pane
	selectedIdx  int
	detailScroll int // Scroll offset for detail pane

	activeFilter parser.Action // "" means all
	searchQuery  string
	searchActive bool

	width  int
	height int

	showHelp        bool
	showRisk        bool
	groupByModule   bool
	expandedModules map[string]bool // Track which modules are expanded

	exportMessage string // Message to show after export
}

func NewModel(p *parser.PlanSummary) Model {
	m := Model{
		plan:            p,
		riskAnalysis:    plan.CalculateRiskScore(p),
		activePane:      PaneResourceList,
		expandedModules: make(map[string]bool),
	}
	m.applyFilter()
	return m
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.searchActive {
			return m.handleSearchKey(msg)
		}
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		if m.showRisk {
			m.showRisk = false
			return m, nil
		}
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.activePane == PaneResourceList {
			if m.selectedIdx > 0 {
				m.selectedIdx--
				m.detailScroll = 0 // Reset detail scroll when changing resource
			}
		} else {
			// Scroll detail pane up
			if m.detailScroll > 0 {
				m.detailScroll--
			}
		}
	case "down", "j":
		if m.activePane == PaneResourceList {
			if m.selectedIdx < len(m.filteredRes)-1 {
				m.selectedIdx++
				m.detailScroll = 0 // Reset detail scroll when changing resource
			}
		} else {
			// Scroll detail pane down
			m.detailScroll++
		}
	case "g":
		if m.activePane == PaneResourceList {
			m.selectedIdx = 0
			m.detailScroll = 0
		} else {
			m.detailScroll = 0
		}
	case "G":
		if m.activePane == PaneResourceList {
			if len(m.filteredRes) > 0 {
				m.selectedIdx = len(m.filteredRes) - 1
				m.detailScroll = 0
			}
		} else {
			// Jump to bottom of detail (will be clamped in render)
			m.detailScroll = 999999
		}
	case "ctrl+u":
		if m.activePane == PaneResourceList {
			half := m.listViewHeight() / 2
			m.selectedIdx -= half
			if m.selectedIdx < 0 {
				m.selectedIdx = 0
			}
			m.detailScroll = 0
		} else {
			half := m.listViewHeight() / 2
			m.detailScroll -= half
			if m.detailScroll < 0 {
				m.detailScroll = 0
			}
		}
	case "ctrl+d":
		if m.activePane == PaneResourceList {
			half := m.listViewHeight() / 2
			m.selectedIdx += half
			if m.selectedIdx >= len(m.filteredRes) {
				m.selectedIdx = len(m.filteredRes) - 1
			}
			if m.selectedIdx < 0 {
				m.selectedIdx = 0
			}
			m.detailScroll = 0
		} else {
			half := m.listViewHeight() / 2
			m.detailScroll += half
		}
	case "tab":
		if m.activePane == PaneResourceList {
			m.activePane = PaneDetail
		} else {
			m.activePane = PaneResourceList
		}
	case "f":
		m.cycleFilter()
		m.detailScroll = 0
	case "0":
		m.setFilter("")
		m.detailScroll = 0
	case "1":
		m.setFilter(parser.ActionCreate)
		m.detailScroll = 0
	case "2":
		m.setFilter(parser.ActionUpdate)
		m.detailScroll = 0
	case "3":
		m.setFilter(parser.ActionDelete)
		m.detailScroll = 0
	case "4":
		m.setFilter(parser.ActionReplace)
		m.detailScroll = 0
	case "/":
		m.searchActive = true
		m.searchQuery = ""
		m.detailScroll = 0
	case "m":
		m.groupByModule = !m.groupByModule
		m.applyFilter() // Rebuild display list
		m.detailScroll = 0
	case "enter":
		if m.activePane == PaneResourceList && m.groupByModule {
			// Toggle module expansion
			if m.selectedIdx >= 0 && m.selectedIdx < len(m.displayList) {
				item := m.displayList[m.selectedIdx]
				if item.isModuleHeader {
					m.expandedModules[item.moduleName] = !m.expandedModules[item.moduleName]
					m.applyFilter() // Rebuild display list
				}
			}
		}
	case "e":
		// Export to markdown
		filename := fmt.Sprintf("tf-plan-summary-%s.md", time.Now().Format("20060102-150405"))
		markdown := plan.ExportMarkdown(m.plan, m.riskAnalysis)
		if err := os.WriteFile(filename, []byte(markdown), 0644); err != nil {
			m.exportMessage = fmt.Sprintf("❌ Export failed: %v", err)
		} else {
			m.exportMessage = fmt.Sprintf("✓ Exported to %s", filename)
		}
	case "r":
		m.showRisk = true
	case "?":
		m.showHelp = true
	}
	return m, nil
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.searchActive = false
	case "esc":
		m.searchActive = false
		m.searchQuery = ""
		m.applyFilter()
	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.applyFilter()
		}
	default:
		if len(msg.String()) == 1 {
			m.searchQuery += msg.String()
			m.applyFilter()
		}
	}
	return m, nil
}

func (m *Model) cycleFilter() {
	order := []parser.Action{"", parser.ActionCreate, parser.ActionUpdate, parser.ActionDelete, parser.ActionReplace}
	for i, a := range order {
		if a == m.activeFilter {
			m.activeFilter = order[(i+1)%len(order)]
			m.applyFilter()
			return
		}
	}
	m.activeFilter = ""
	m.applyFilter()
}

func (m *Model) setFilter(a parser.Action) {
	m.activeFilter = a
	m.applyFilter()
}

func (m *Model) applyFilter() {
	m.filteredRes = nil
	for _, r := range m.plan.Resources {
		if r.Action == parser.ActionNoOp || r.Action == parser.ActionRead {
			if m.activeFilter != r.Action {
				continue
			}
		}
		if m.activeFilter != "" && r.Action != m.activeFilter {
			continue
		}
		if m.searchQuery != "" {
			if !strings.Contains(strings.ToLower(r.Address), strings.ToLower(m.searchQuery)) {
				continue
			}
		}
		m.filteredRes = append(m.filteredRes, r)
	}

	// Build display list
	m.buildDisplayList()

	if m.selectedIdx >= len(m.displayList) {
		m.selectedIdx = len(m.displayList) - 1
	}
	if m.selectedIdx < 0 {
		m.selectedIdx = 0
	}
}

func (m *Model) buildDisplayList() {
	m.displayList = nil

	if !m.groupByModule {
		// Flat list
		for i := range m.filteredRes {
			m.displayList = append(m.displayList, displayItem{
				resource:     &m.filteredRes[i],
				displayIndex: i,
			})
		}
		return
	}

	// Grouped by module
	modules := make(map[string][]int) // module -> indices in filteredRes
	var moduleOrder []string

	for i, r := range m.filteredRes {
		mod := r.ModuleAddress
		if mod == "" {
			mod = "root"
		}
		if _, exists := modules[mod]; !exists {
			moduleOrder = append(moduleOrder, mod)
		}
		modules[mod] = append(modules[mod], i)
	}

	// Build display list with module headers
	for _, mod := range moduleOrder {
		indices := modules[mod]

		// Count by action
		stats := make(map[parser.Action]int)
		for _, idx := range indices {
			stats[m.filteredRes[idx].Action]++
		}

		// Format stats string
		var parts []string
		if c := stats[parser.ActionCreate]; c > 0 {
			parts = append(parts, fmt.Sprintf("%d create", c))
		}
		if c := stats[parser.ActionUpdate]; c > 0 {
			parts = append(parts, fmt.Sprintf("%d update", c))
		}
		if c := stats[parser.ActionDelete]; c > 0 {
			parts = append(parts, fmt.Sprintf("%d delete", c))
		}
		if c := stats[parser.ActionReplace]; c > 0 {
			parts = append(parts, fmt.Sprintf("%d replace", c))
		}
		statsStr := strings.Join(parts, ", ")

		// Add module header
		m.displayList = append(m.displayList, displayItem{
			isModuleHeader: true,
			moduleName:     mod,
			moduleStats:    fmt.Sprintf("%d changes: %s", len(indices), statsStr),
		})

		// Add resources if expanded
		if m.expandedModules[mod] {
			for _, idx := range indices {
				m.displayList = append(m.displayList, displayItem{
					resource:     &m.filteredRes[idx],
					displayIndex: idx,
				})
			}
		}
	}
}

func (m Model) listViewHeight() int {
	// header(2) + filter(1) + footer(2) + borders(4)
	h := m.height - 9
	if h < 3 {
		h = 3
	}
	return h
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	if m.showHelp {
		return m.renderHelp()
	}

	if m.showRisk {
		return m.renderRisk()
	}

	header := m.renderHeader()
	filterBar := m.renderFilterBar()
	body := m.renderBody()
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, header, filterBar, body, footer)
}

func (m Model) renderHeader() string {
	s := m.plan.Stats
	riskIcon := plan.RiskIcon(m.riskAnalysis.Level)
	riskColor := riskColorStyle(m.riskAnalysis.Level)

	title := HeaderStyle.Render(fmt.Sprintf("tf-plan-tui  Plan: %d changes  Risk: %s %s %d/100  Terraform %s",
		s.TotalChanges,
		riskIcon,
		riskColor.Render(strings.ToUpper(m.riskAnalysis.Level)),
		m.riskAnalysis.Score,
		m.plan.TerraformVersion))

	counts := fmt.Sprintf(" %s %d create  %s %d update  %s %d delete  %s %d replace",
		ActionStyle(parser.ActionCreate).Render("+"), s.Creates,
		ActionStyle(parser.ActionUpdate).Render("~"), s.Updates,
		ActionStyle(parser.ActionDelete).Render("-"), s.Deletes,
		ActionStyle(parser.ActionReplace).Render("±"), s.Replaces,
	)

	return lipgloss.JoinVertical(lipgloss.Left, title, counts)
}

func (m Model) renderFilterBar() string {
	filters := []struct {
		label  string
		action parser.Action
	}{
		{"All", ""},
		{"Create", parser.ActionCreate},
		{"Update", parser.ActionUpdate},
		{"Delete", parser.ActionDelete},
		{"Replace", parser.ActionReplace},
	}

	var parts []string
	for _, f := range filters {
		label := f.label
		if f.action == m.activeFilter {
			parts = append(parts, FilterActiveStyle.Render(label))
		} else {
			parts = append(parts, FilterInactiveStyle.Render(label))
		}
	}

	bar := " Filter: " + strings.Join(parts, " ")
	if m.searchActive {
		bar += fmt.Sprintf("  / %s_", m.searchQuery)
	} else if m.searchQuery != "" {
		bar += fmt.Sprintf("  search: %q (%d matches)", m.searchQuery, len(m.filteredRes))
	}

	// Show export message if set
	if m.exportMessage != "" {
		bar += "  " + m.exportMessage
	}

	return bar
}

func (m Model) renderBody() string {
	bodyH := m.listViewHeight()

	// Calculate widths - account for borders (2 chars each side + 2 for spacing)
	listW := m.width/3 - 4
	if listW < 20 {
		listW = 20
	}
	detailW := m.width - listW - 8
	if detailW < 20 {
		detailW = 20
	}

	list := m.renderResourceList(listW, bodyH)
	detail := m.renderDetail(detailW, bodyH)

	var listBox, detailBox string
	if m.activePane == PaneResourceList {
		listBox = ActivePaneBorder.Width(listW).Height(bodyH).Render(list)
		detailBox = InactivePaneBorder.Width(detailW).Height(bodyH).Render(detail)
	} else {
		listBox = InactivePaneBorder.Width(listW).Height(bodyH).Render(list)
		detailBox = ActivePaneBorder.Width(detailW).Height(bodyH).Render(detail)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, listBox, detailBox)
}

func (m Model) renderResourceList(w, h int) string {
	if len(m.displayList) == 0 {
		return padLines([]string{"  No resources match"}, w, h)
	}

	// Determine visible window
	start := 0
	if m.selectedIdx >= h {
		start = m.selectedIdx - h + 1
	}
	end := start + h
	if end > len(m.displayList) {
		end = len(m.displayList)
	}

	var lines []string
	for i := start; i < end; i++ {
		item := m.displayList[i]
		var line string

		if item.isModuleHeader {
			// Module header
			icon := "▾"
			if !m.expandedModules[item.moduleName] {
				icon = "▸"
			}
			line = fmt.Sprintf(" %s %s (%s)", icon, item.moduleName, item.moduleStats)
		} else {
			// Resource
			r := item.resource
			icon := ActionStyle(r.Action).Render(parser.ActionIcon(r.Action))
			addr := truncate(r.Address, w-4)
			if m.groupByModule {
				// Indent resources under modules
				line = fmt.Sprintf("   %s %s", icon, addr)
			} else {
				line = fmt.Sprintf(" %s %s", icon, addr)
			}
		}

		if i == m.selectedIdx {
			line = SelectedRowStyle.Width(w).Render(line)
		} else {
			line = lipgloss.NewStyle().Width(w).Render(line)
		}
		lines = append(lines, line)
	}

	// Add scroll indicators
	if start > 0 {
		indicator := lipgloss.NewStyle().Foreground(ColorMuted).Width(w).Align(lipgloss.Center).Render("↑ more")
		lines[0] = indicator
	}
	if end < len(m.displayList) {
		indicator := lipgloss.NewStyle().Foreground(ColorMuted).Width(w).Align(lipgloss.Center).Render("↓ more")
		if len(lines) > 0 {
			lines[len(lines)-1] = indicator
		}
	}

	return padLines(lines, w, h)
}

func (m Model) renderDetail(w, h int) string {
	if len(m.displayList) == 0 {
		return padLines([]string{"  Select a resource"}, w, h)
	}

	item := m.displayList[m.selectedIdx]
	if item.isModuleHeader {
		// Show module summary instead of resource details
		lines := []string{
			fmt.Sprintf(" Module: %s", item.moduleName),
			" " + strings.Repeat("─", min(w-2, 80)),
			"",
			fmt.Sprintf(" %s", item.moduleStats),
			"",
			" Press Enter to expand/collapse",
		}
		return m.clipDetailLines(lines, w, h)
	}

	r := item.resource

	// Generate all detail lines
	var lines []string

	// Header section
	lines = append(lines, truncate(fmt.Sprintf(" %s (%s)", r.Address, strings.ToUpper(string(r.Action))), w))
	if r.ModuleAddress != "" {
		lines = append(lines, truncate(fmt.Sprintf(" Module: %s", r.ModuleAddress), w))
	}
	lines = append(lines, truncate(fmt.Sprintf(" Provider: %s", shortProvider(r.ProviderName)), w))
	if r.ActionReason != "" {
		lines = append(lines, truncate(fmt.Sprintf(" Reason: %s", r.ActionReason), w))
	}
	lines = append(lines, " "+strings.Repeat("─", min(w-2, 80)))

	// Content section
	if len(r.AttrChanges) == 0 {
		lines = append(lines, "")
		if r.Action == parser.ActionCreate {
			lines = append(lines, " New resource (all attributes set on apply)")
		} else if r.Action == parser.ActionDelete {
			lines = append(lines, " Resource will be destroyed")
		} else {
			lines = append(lines, " No attribute changes")
		}
	} else {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf(" Changed attributes (%d):", len(r.AttrChanges)))
		for _, attr := range r.AttrChanges {
			// Check if security-relevant
			isSecure := plan.IsSecurityRelevantChange(*r, attr)
			pathLine := fmt.Sprintf("   %s:", attr.Path)
			if isSecure {
				pathLine += " ⚠️  SECURITY"
			}
			lines = append(lines, truncate(pathLine, w))

			if attr.IsSensitive {
				lines = append(lines, "     (sensitive)")
				continue
			}
			switch attr.Action {
			case "changed":
				beforeLine := truncate(fmt.Sprintf("     - %s", attr.Before), w)
				afterLine := truncate(fmt.Sprintf("     + %s", attr.After), w)
				if isSecure {
					// Highlight security-relevant changes with brighter colors
					lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#fca5a5"}).Bold(true).Render(beforeLine))
					lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#fca5a5"}).Bold(true).Render(afterLine))
				} else {
					lines = append(lines, DiffRemoveStyle.Render(beforeLine))
					lines = append(lines, DiffAddStyle.Render(afterLine))
				}
			case "added":
				afterLine := truncate(fmt.Sprintf("     + %s", attr.After), w)
				if isSecure {
					lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#fca5a5"}).Bold(true).Render(afterLine))
				} else {
					lines = append(lines, DiffAddStyle.Render(afterLine))
				}
			case "removed":
				beforeLine := truncate(fmt.Sprintf("     - %s", attr.Before), w)
				if isSecure {
					lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#fca5a5"}).Bold(true).Render(beforeLine))
				} else {
					lines = append(lines, DiffRemoveStyle.Render(beforeLine))
				}
			}
		}
	}

	// Unknown after apply section
	if len(r.AfterUnknown) > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf(" Unknown after apply (%d):", len(r.AfterUnknown)))
		for k := range r.AfterUnknown {
			lines = append(lines, truncate(fmt.Sprintf("   ~ %s", k), w))
		}
	}

	// Apply scrolling and clipping
	return m.clipDetailLines(lines, w, h)
}

func (m *Model) clipDetailLines(lines []string, w, h int) string {
	totalLines := len(lines)

	// Clamp scroll offset
	maxScroll := totalLines - h
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.detailScroll > maxScroll {
		m.detailScroll = maxScroll
	}
	if m.detailScroll < 0 {
		m.detailScroll = 0
	}

	// Extract visible window
	start := m.detailScroll
	end := start + h
	if end > totalLines {
		end = totalLines
	}

	visible := lines[start:end]

	// Add scroll indicators
	if start > 0 && len(visible) > 0 {
		visible[0] = lipgloss.NewStyle().Foreground(ColorMuted).Width(w).Align(lipgloss.Center).Render("↑ more")
	}
	if end < totalLines && len(visible) > 0 {
		visible[len(visible)-1] = lipgloss.NewStyle().Foreground(ColorMuted).Width(w).Align(lipgloss.Center).Render("↓ more")
	}

	return padLines(visible, w, h)
}

func (m Model) renderFooter() string {
	var left string
	if m.activePane == PaneDetail {
		left = " j/k:scroll  Tab:switch  e:export  r:risk  ?:help  q:quit"
	} else {
		if m.groupByModule {
			left = " j/k:navigate  Enter:expand  e:export  r:risk  ?:help  q:quit"
		} else {
			left = " j/k:navigate  Tab:switch  e:export  r:risk  ?:help  q:quit"
		}
	}
	return FooterStyle.Width(m.width).Render(left)
}

func (m Model) renderHelp() string {
	help := `
  tf-plan-tui Help

  Navigation
  j/k or Up/Down   Move up/down (scroll detail when focused)
  g/G              Top/bottom
  Ctrl+U/D         Half-page up/down
  Tab              Switch panes (list ↔ detail)
  Enter            Expand/collapse module (when grouped)

  Detail Pane
  When focused (Tab), use j/k to scroll through long content
  Scroll indicators (↑/↓ more) show when content overflows

  Filtering & Grouping
  f                Cycle filters
  0                Show all
  1                Creates only
  2                Updates only
  3                Deletes only
  4                Replaces only
  /                Search (Esc to cancel)
  m                Toggle module grouping

  Analysis & Export
  r                Show risk analysis
  e                Export as markdown

  q / Ctrl+C       Quit
  ?                Close this help
`
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorActiveBorder).
		Padding(1, 2).
		Width(48).
		Render(help)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func truncate(s string, max int) string {
	if max <= 3 {
		max = 3
	}
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}

func shortProvider(p string) string {
	parts := strings.Split(p, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return p
}

// padLines ensures lines array is exactly h lines tall, padding with empty lines if needed.
func padLines(lines []string, w, h int) string {
	result := make([]string, h)
	plainStyle := lipgloss.NewStyle().Width(w)

	for i := 0; i < h; i++ {
		if i < len(lines) {
			// Only apply width if the line doesn't already have styling
			// Check if line contains ANSI codes (styled content)
			if strings.Contains(lines[i], "\x1b[") {
				result[i] = lines[i]
			} else {
				result[i] = plainStyle.Render(lines[i])
			}
		} else {
			result[i] = plainStyle.Render("")
		}
	}
	return strings.Join(result, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func riskColorStyle(level string) lipgloss.Style {
	var color lipgloss.AdaptiveColor
	switch level {
	case "critical":
		color = lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#ef4444"} // Red
	case "high":
		color = lipgloss.AdaptiveColor{Light: "#ea580c", Dark: "#f97316"} // Orange
	case "medium":
		color = lipgloss.AdaptiveColor{Light: "#ca8a04", Dark: "#eab308"} // Yellow
	case "low":
		color = lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#22c55e"} // Green
	default:
		color = ColorMuted
	}
	return lipgloss.NewStyle().Foreground(color).Bold(true)
}

func (m Model) renderRisk() string {
	ra := m.riskAnalysis
	var content strings.Builder

	content.WriteString("\n  Risk Analysis\n\n")
	content.WriteString(fmt.Sprintf("  Score: %s %s %d/100\n\n",
		plan.RiskIcon(ra.Level),
		riskColorStyle(ra.Level).Render(strings.ToUpper(ra.Level)),
		ra.Score))

	if len(ra.Reasons) == 0 {
		content.WriteString("  No significant risk factors identified.\n")
	} else {
		content.WriteString(fmt.Sprintf("  Risk Factors (%d):\n\n", len(ra.Reasons)))
		for i, reason := range ra.Reasons {
			if i >= 15 {
				content.WriteString(fmt.Sprintf("  ... and %d more\n", len(ra.Reasons)-15))
				break
			}
			content.WriteString(fmt.Sprintf("  • %s\n", reason))
		}
	}

	content.WriteString("\n  Press any key to close\n")

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(riskColorStyleBorder(ra.Level)).
		Padding(1, 2).
		Width(min(80, m.width-4)).
		Render(content.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func riskColorStyleBorder(level string) lipgloss.AdaptiveColor {
	switch level {
	case "critical":
		return lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#ef4444"}
	case "high":
		return lipgloss.AdaptiveColor{Light: "#ea580c", Dark: "#f97316"}
	case "medium":
		return lipgloss.AdaptiveColor{Light: "#ca8a04", Dark: "#eab308"}
	case "low":
		return lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#22c55e"}
	default:
		return ColorBorder
	}
}
