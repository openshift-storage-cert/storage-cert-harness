// Package report renders a run's verdicts into machine- and human-readable
// forms. Provenance is stamped into every report (5273 / decisions/0004).
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
)

// Build assembles a Report from verdicts and provenance, computing the summary,
// the per-level rollup, and the certified level. Each verdict's Level (stamped by
// the orchestrator) drives the rollup. See ADR-0014.
func Build(runID, schemaVersion string, prov core.Provenance, verdicts []core.Verdict) core.Report {
	r := core.Report{RunID: runID, SchemaVersion: schemaVersion, Provenance: prov, Verdicts: verdicts}
	r.Summary.Total = len(verdicts)
	for _, v := range verdicts {
		switch v.Outcome {
		case core.OutcomePass:
			r.Summary.Pass++
		case core.OutcomeFail:
			r.Summary.Fail++
		case core.OutcomeError:
			r.Summary.Error++
		case core.OutcomeSkip:
			r.Summary.Skip++
		}
	}
	r.Levels, r.Summary.CertifiedLevel = rollupLevels(verdicts)
	return r
}

// rollupLevels groups verdicts by partner level and computes the certified level.
//
// A level passes iff it has at least one pass and no fail/error; a skip-only level
// is "not evaluated". CertifiedLevel is the highest *tested* level that passed,
// with no lower tested level failing; levels that were not tested are ignored (not
// treated as failures). Today every harness TR is Level 2, and Level 1 is a
// separate external suite (openshift-origin-csi), so a strict cumulative ladder
// across all three levels is intentionally NOT enforced here — see ADR-0014.
func rollupLevels(verdicts []core.Verdict) ([]core.LevelRollup, int) {
	type acc struct{ pass, failErr bool }
	per := map[int]*acc{}
	for _, v := range verdicts {
		lvl := v.Level
		if lvl == 0 {
			continue
		}
		a := per[lvl]
		if a == nil {
			a = &acc{}
			per[lvl] = a
		}
		switch v.Outcome {
		case core.OutcomePass:
			a.pass = true
		case core.OutcomeFail, core.OutcomeError:
			a.failErr = true
		}
	}
	if len(per) == 0 {
		return nil, 0
	}
	levels := make([]int, 0, len(per))
	for l := range per {
		levels = append(levels, l)
	}
	sort.Ints(levels)

	outcome := func(a *acc) core.Outcome {
		switch {
		case a.failErr:
			return core.OutcomeFail
		case a.pass:
			return core.OutcomePass
		default:
			return core.OutcomeSkip
		}
	}
	var rollups []core.LevelRollup
	for _, l := range levels {
		rollups = append(rollups, core.LevelRollup{Level: l, Outcome: outcome(per[l])})
	}
	// Walk tested levels ascending: a pass raises the certified level; a fail/error
	// at a tested level stops the ladder; a skip-only level is ignored.
	certified := 0
	for _, l := range levels {
		switch outcome(per[l]) {
		case core.OutcomePass:
			certified = l
		case core.OutcomeFail:
			return rollups, certified
		}
	}
	return rollups, certified
}

// WriteJSON renders the report as indented JSON.
func WriteJSON(w io.Writer, r core.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false) // keep "<=" readable in reports
	return enc.Encode(r)
}

// WriteMarkdown renders a human-readable summary.
func WriteMarkdown(w io.Writer, r core.Report) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# Certification report %s\n\n", r.RunID)
	fmt.Fprintf(&b, "- Provenance: schema %s", r.SchemaVersion)
	if r.Provenance.KBGitCommit != "" {
		fmt.Fprintf(&b, ", KB commit %s", r.Provenance.KBGitCommit)
	}
	if r.Provenance.GeneratedDate != "" {
		fmt.Fprintf(&b, ", generated %s", r.Provenance.GeneratedDate)
	}
	b.WriteString("\n")
	if p := r.Environment.Platform; p.OCPVersion != "" || p.CSIDriver.Name != "" {
		fmt.Fprintf(&b, "- Environment: OCP %s, CNV %s, driver %s (%d nodes)\n",
			p.OCPVersion, p.CNVVersion, p.CSIDriver.Name, r.Environment.Hardware.Nodes)
	}
	fmt.Fprintf(&b, "- Certified level: %d\n", r.Summary.CertifiedLevel)
	fmt.Fprintf(&b, "- Totals: %d pass / %d fail / %d error / %d skip (of %d)\n\n",
		r.Summary.Pass, r.Summary.Fail, r.Summary.Error, r.Summary.Skip, r.Summary.Total)
	b.WriteString("| TR | Item | Outcome | Duration | Expected | Actual | Reason |\n")
	b.WriteString("|----|------|---------|----------|----------|--------|--------|\n")
	for _, v := range r.Verdicts {
		fmt.Fprintf(&b, "| %s | %s | %s | %.1fs | %s | %s | %s |\n",
			v.TR, v.Item, v.Outcome, v.DurationS, v.Expected, v.Actual, v.Reason)
	}

	// Measured values are always shown, whether or not an SLA graded them (ADR-0014).
	if len(r.Measurements) > 0 {
		b.WriteString("\n## Measurements\n\n")
		b.WriteString("| TR | Metric | Percentile | Value | Unit |\n")
		b.WriteString("|----|--------|------------|-------|------|\n")
		for _, m := range r.Measurements {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
				m.TR, m.Name, m.Percentile, strconv.FormatFloat(m.Value, 'f', -1, 64), m.Unit)
		}
	}

	_, err := io.WriteString(w, b.String())
	return err
}
