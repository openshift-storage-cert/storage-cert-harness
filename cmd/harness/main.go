// Command harness is the CSI/storage certification test harness CLI.
//
// Tool integrations are registered by blank-importing their packages below —
// that single import line is all it takes to add a compiled-in tool. See
// decisions/0003.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/backend"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/catalog"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/clustercheck"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/config"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/kb"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/orchestrator"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/plan"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/registry"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/report"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/thresholds"

	// Tool integrations — add a line here to compile in a new tool.
	_ "gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/tools/example"
	_ "gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/tools/kubeburner"
	_ "gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/tools/kubeburnerocp"
	_ "gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/tools/virtbench"
)

// version is overridable at build time: -ldflags "-X main.version=..."
var version = "0.0.0-dev"

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err) //nolint:errcheck // follow-up: check CLI writes
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "harness",
		Short:         "Storage certification test harness",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCmd(), newListCmd(), newValidateCmd(), newPreflightCmd(), newRunCmd())
	return root
}

func newLogger(verbose bool) *slog.Logger {
	lvl := slog.LevelInfo
	if verbose {
		lvl = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the harness version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), version) //nolint:errcheck // follow-up: check CLI writes
			return nil
		},
	}
}

func newListCmd() *cobra.Command {
	var catalogPath string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List registered tools and (optionally) catalog test requirements",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "Registered tools:") //nolint:errcheck // follow-up: check CLI writes
			for _, name := range registry.Names() {
				ti, _ := registry.Get(name)
				fmt.Fprintf(out, "  - %s (image: %s) provides %v\n", ti.Name, ti.Image, ti.Provides) //nolint:errcheck // follow-up: check CLI writes
			}
			if catalogPath != "" {
				cat, err := catalog.Load(catalogPath)
				if err != nil {
					return err
				}
				fmt.Fprintf(out, "\nCatalog test requirements (%d), schema %s, KB commit %s:\n", //nolint:errcheck // follow-up: check CLI writes
					len(cat.TestRequirements), cat.SchemaVersion, cat.Provenance.KBGitCommit)
				for _, tr := range cat.TestRequirements {
					fmt.Fprintf(out, "  - %s L%d [tool=%s] %q (sla=%d/%s, checks=%d/%s)\n", //nolint:errcheck // follow-up: check CLI writes
						tr.ID, tr.PartnerLevel, tr.AutomationTool, tr.Title,
						tr.SLACount, tr.SLAStatus, tr.ChecksCount, tr.ChecksStatus)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&catalogPath, "catalog", "", "path to catalog.json (publishable)")
	return cmd
}

func newValidateCmd() *cobra.Command {
	var catalogPath, thresholdsPath string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a catalog and (optionally) a thresholds bundle, verifying they join",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			cat, err := catalog.Load(catalogPath)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "catalog OK: %d test requirements, schema %s, KB commit %s\n", //nolint:errcheck // follow-up: check CLI writes
				len(cat.TestRequirements), cat.SchemaVersion, cat.Provenance.KBGitCommit)
			if thresholdsPath != "" {
				th, err := thresholds.Load(thresholdsPath)
				if err != nil {
					return err
				}
				fmt.Fprintf(out, "thresholds OK: %d test requirements, schema %s, KB commit %s\n", //nolint:errcheck // follow-up: check CLI writes
					len(th.TestRequirements), th.SchemaVersion, th.Provenance.KBGitCommit)
				trs, err := kb.Join(cat, th)
				if err != nil {
					return err
				}
				fmt.Fprintf(out, "join OK: %d joined test requirements (catalog + thresholds agree)\n", len(trs)) //nolint:errcheck // follow-up: check CLI writes
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&catalogPath, "catalog", "", "path to catalog.json (required)")
	cmd.Flags().StringVar(&thresholdsPath, "thresholds", "", "path to thresholds.json (optional)")
	_ = cmd.MarkFlagRequired("catalog")
	return cmd
}

func newPreflightCmd() *cobra.Command {
	var (
		catalogPath  string
		filter       config.Filter
		planPath     string
		backendsPath string
		backendName  string
	)
	var verbose bool
	cmd := &cobra.Command{
		Use:   "preflight",
		Short: "Run preflight checks for the tools selected by the catalog/filter",
		RunE: func(cmd *cobra.Command, _ []string) error {
			logger := newLogger(verbose)
			cfg := config.Config{Filter: filter}
			resolved, err := configureExecution(cmd, &cfg, planPath, backendsPath, &backendName, false, logger)
			if err != nil {
				return err
			}
			cat, err := catalog.Load(catalogPath)
			if err != nil {
				return err
			}
			trs := orchestrator.ApplyOverrides(orchestrator.SelectTRs(kb.FromCatalog(cat), cfg.Filter), cfg.Overrides)
			rc := &core.RunCtx{RunID: "preflight", Logger: logger, Backend: resolved}
			out := cmd.OutOrStdout()
			byTool := map[string][]core.TestRequirement{}
			for _, tr := range trs {
				byTool[tr.AutomationTool] = append(byTool[tr.AutomationTool], tr)
			}
			var problems int
			for tool, ts := range byTool {
				if tool == "" || tool == core.CheckManual {
					fmt.Fprintf(out, "[info] %d TR(s) with no automation tool: skipped\n", len(ts)) //nolint:errcheck // follow-up: check CLI writes
					continue
				}
				ti, ok := orchestrator.ResolveTool(ts[0])
				if !ok {
					fmt.Fprintf(out, "[error] tool %q not registered\n", tool) //nolint:errcheck // follow-up: check CLI writes
					problems++
					continue
				}
				if ti.Preflight == nil {
					fmt.Fprintf(out, "[info] %s: no preflight\n", tool) //nolint:errcheck // follow-up: check CLI writes
					continue
				}
				findings, err := ti.Preflight.Check(cmd.Context(), rc, core.NewBag(), ts)
				if err != nil {
					fmt.Fprintf(out, "[error] %s: %v\n", tool, err) //nolint:errcheck // follow-up: check CLI writes
					problems++
					continue
				}
				for _, f := range findings {
					fmt.Fprintf(out, "[%s] %s: %s\n", f.Level, tool, f.Message) //nolint:errcheck // follow-up: check CLI writes
					if f.Level == "error" {
						problems++
					}
				}
			}
			if problems > 0 {
				return fmt.Errorf("%d preflight problem(s)", problems)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&catalogPath, "catalog", "", "path to catalog.json (required)")
	cmd.Flags().StringVar(&planPath, "plan", "", "path to a test plan (YAML); provides selection/backend defaults")
	cmd.Flags().StringVar(&backendsPath, "backends", "", "path to a backends file (YAML)")
	cmd.Flags().StringVar(&backendName, "backend", "", "name of the active storage backend (overrides the plan)")
	cmd.Flags().StringSliceVar(&filter.Tools, "tool", nil, "restrict to these automation tools")
	cmd.Flags().StringSliceVar(&filter.IDs, "id", nil, "restrict to these TR ids")
	cmd.Flags().IntSliceVar(&filter.PartnerLevels, "partner-level", nil, "restrict to these partner levels (1|2|3)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "debug logging")
	_ = cmd.MarkFlagRequired("catalog")
	return cmd
}

func newRunCmd() *cobra.Command {
	var (
		cfg              config.Config
		thresholdsPath   string
		outputDir        string
		verbose          bool
		planPath         string
		backendsPath     string
		backendName      string
		attestationsPath string
	)
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run selected certification tests and emit a report",
		RunE: func(cmd *cobra.Command, _ []string) error {
			logger := newLogger(verbose)
			resolved, err := configureExecution(cmd, &cfg, planPath, backendsPath, &backendName, true, logger)
			if err != nil {
				return err
			}

			cat, err := catalog.Load(cfg.CatalogPath)
			if err != nil {
				return err
			}

			// Resolve thresholds: --thresholds flag, then HARNESS_THRESHOLDS.
			path := thresholdsPath
			if path == "" {
				path = os.Getenv("HARNESS_THRESHOLDS")
			}

			// Build the execution TRs: join if a thresholds bundle is supplied
			// (verifies provenance/commit match + id parity), else catalog-only.
			var trs []core.TestRequirement
			if path != "" {
				th, err := thresholds.Load(path)
				if err != nil {
					return err
				}
				trs, err = kb.Join(cat, th)
				if err != nil {
					return err
				}
			} else {
				trs = kb.FromCatalog(cat)
			}

			selected := orchestrator.SelectTRs(trs, cfg.Filter)
			if len(selected) == 0 {
				return fmt.Errorf("no test requirements selected")
			}
			// Refuse a run that needs SLA/check numbers without a bundle (5273).
			if path == "" {
				for _, tr := range selected {
					if tr.SLACount > 0 || tr.ChecksCount > 0 {
						return fmt.Errorf("TR %q has scorable criteria (%d sla, %d checks); supply --thresholds or HARNESS_THRESHOLDS",
							tr.ID, tr.SLACount, tr.ChecksCount)
					}
				}
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			runID := time.Now().UTC().Format("20060102-150405")
			workdir := cfg.WorkDir
			if workdir == "" {
				workdir = filepath.Join(os.TempDir(), "harness-"+runID)
			}
			rc := &core.RunCtx{RunID: runID, WorkDir: workdir, Logger: logger, Backend: resolved}

			rep, err := orchestrator.Run(ctx, cfg, trs, cat.Provenance, cat.SchemaVersion, rc)
			if err != nil {
				return err
			}

			// Enrich the report with the run environment (auto-collected from the
			// cluster) and partner attestations (self-reported). Both best-effort;
			// see ADR-0014. The CSI driver is resolved from the StorageClass under
			// test (the active backend), not the cluster default.
			scUnderTest := ""
			if resolved != nil {
				scUnderTest = resolved.StorageClass
			}
			if cli := clustercheck.KubeCLI(); cli != "" {
				rep.Environment = clustercheck.CollectEnvironment(ctx, cli, scUnderTest)
			}
			if resolved != nil {
				rep.Environment.Platform.Backend = resolved.Name
			}
			if attestationsPath != "" {
				atts, err := loadAttestations(attestationsPath)
				if err != nil {
					return err
				}
				rep.Attestations = atts
			}

			if outputDir != "" {
				if err := os.MkdirAll(outputDir, 0o755); err != nil {
					return err
				}
				if err := writeFile(filepath.Join(outputDir, "report.json"), func(f *os.File) error { return report.WriteJSON(f, rep) }); err != nil {
					return err
				}
				if err := writeFile(filepath.Join(outputDir, "report.md"), func(f *os.File) error { return report.WriteMarkdown(f, rep) }); err != nil {
					return err
				}
				if err := writeFile(filepath.Join(outputDir, "report.junit.xml"), func(f *os.File) error { return (report.JUnitExporter{}).Export(f, rep) }); err != nil {
					return err
				}
			}
			if err := report.WriteMarkdown(cmd.OutOrStdout(), rep); err != nil {
				return err
			}

			if rep.Summary.Fail > 0 || rep.Summary.Error > 0 {
				return fmt.Errorf("certification failed: %d fail, %d error", rep.Summary.Fail, rep.Summary.Error)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cfg.CatalogPath, "catalog", "", "path to catalog.json (required)")
	cmd.Flags().StringVar(&thresholdsPath, "thresholds", "", "path to real thresholds.json (or set HARNESS_THRESHOLDS)")
	cmd.Flags().StringVar(&cfg.WorkDir, "workdir", "", "working directory for run artifacts")
	cmd.Flags().IntVar(&cfg.Concurrency, "concurrency", 1, "max tools run in parallel (respects per-tool parallel_safe/exclusivity_groups)")
	cmd.Flags().StringVar(&outputDir, "output", "", "directory to write report.json / report.md")
	cmd.Flags().StringVar(&planPath, "plan", "", "path to a test plan (YAML); provides selection/concurrency/backend defaults")
	cmd.Flags().StringVar(&backendsPath, "backends", "", "path to a backends file (YAML)")
	cmd.Flags().StringVar(&backendName, "backend", "", "name of the active storage backend (overrides the plan)")
	cmd.Flags().StringVar(&attestationsPath, "attestations", "", "path to attestation JSON (self-reported, unobservable claims)")
	cmd.Flags().StringSliceVar(&cfg.Filter.Tools, "tool", nil, "restrict to these automation tools")
	cmd.Flags().StringSliceVar(&cfg.Filter.IDs, "id", nil, "restrict to these TR ids")
	cmd.Flags().IntSliceVar(&cfg.Filter.PartnerLevels, "partner-level", nil, "restrict to these partner levels (1|2|3)")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "debug logging")
	_ = cmd.MarkFlagRequired("catalog")
	return cmd
}

func writeFile(path string, fn func(*os.File) error) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // follow-up: check Close error
	return fn(f)
}

// loadAttestations reads a partner attestation file of the form
// {"signed_by": "...", "claims": {"<claim>": "<value>", ...}} into sorted
// core.Attestation entries. These are self-reported, unobservable claims only
// (ADR-0014).
func loadAttestations(path string) ([]core.Attestation, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("attestations: %w", err)
	}
	var doc struct {
		SignedBy string            `json:"signed_by"`
		Claims   map[string]string `json:"claims"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("attestations: %w", err)
	}
	keys := make([]string, 0, len(doc.Claims))
	for k := range doc.Claims {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]core.Attestation, 0, len(keys))
	for _, k := range keys {
		out = append(out, core.Attestation{Claim: k, Value: doc.Claims[k], SignedBy: doc.SignedBy})
	}
	return out, nil
}

func configureExecution(cmd *cobra.Command, cfg *config.Config, planPath, backendsPath string, backendName *string, applyConcurrency bool, logger *slog.Logger) (*core.ResolvedBackend, error) {
	if planPath != "" {
		p, err := plan.Load(planPath)
		if err != nil {
			return nil, err
		}
		if !cmd.Flags().Changed("tool") && !cmd.Flags().Changed("id") && !cmd.Flags().Changed("partner-level") {
			cfg.Filter = p.Filter()
		}
		if applyConcurrency && !cmd.Flags().Changed("concurrency") && p.Concurrency > 0 {
			cfg.Concurrency = p.Concurrency
		}
		if !cmd.Flags().Changed("backend") && p.Backend != "" {
			*backendName = p.Backend
		}
		cfg.Overrides = p.Overrides
		cfg.Scheduling = p.SchedulingOverrides()
		logger.Info("loaded plan", "name", p.Name, "path", planPath)
	}
	if *backendName == "" {
		return nil, nil
	}
	if backendsPath == "" {
		return nil, fmt.Errorf("backend %q selected but no --backends file given", *backendName)
	}
	set, err := backend.Load(backendsPath)
	if err != nil {
		return nil, err
	}
	b, ok := set.Get(*backendName)
	if !ok {
		return nil, fmt.Errorf("backend %q not found in %s", *backendName, backendsPath)
	}
	resolved, err := backend.Resolve(b, nil, logger)
	if err != nil {
		return nil, err
	}
	logger.Info("using backend", "name", resolved.Name, "vendor", resolved.Vendor, "storage_class", resolved.StorageClass)
	return resolved, nil
}
