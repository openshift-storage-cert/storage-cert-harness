// Package config holds run configuration assembled from CLI flags, a plan, and
// env. See decisions/0005 (plans/backends) and 0006 (KB v2.0).
package config

// Filter narrows which test requirements a run selects. Empty fields match all;
// fields are ANDed together, values within a field are ORed.
type Filter struct {
	Tools         []string
	IDs           []string // TR ids
	PartnerLevels []int
}

// ToolSchedule overrides a tool's built-in scheduling defaults for one run.
type ToolSchedule struct {
	ParallelSafe      *bool // nil => keep the ToolIntegration default
	ExclusivityGroups []string
}

// Config is the resolved configuration for a run.
type Config struct {
	CatalogPath    string
	ThresholdsPath string // sensitive bundle; may be empty for catalog-only/list runs
	WorkDir        string
	Concurrency    int
	Filter         Filter

	// Overrides are per-TR tool param overrides (TR id -> params), merged onto
	// the TR before the run. See decisions/0005.
	Overrides map[string]map[string]any

	// Scheduling overrides a tool's built-in parallel-safety/exclusivity for
	// this run (tool name -> override). See decisions/0005 + 0006.
	Scheduling map[string]ToolSchedule
}
