package grader

import "strings"

// This file makes SLA comparison unit-aware. The KB expresses a gate in whatever
// unit reads naturally in the TR prose (e.g. vm_boot_time < 10 min), while a tool
// emits whatever unit it measures in (virtbench reports seconds). Comparing the
// raw floats — 480 vs 10 — is a latent correctness bug: it fails a passing run
// and passes a failing one. We canonicalize both sides to a common base unit
// before comparing, while still displaying the original units in the report.

// unitConv is a unit's multiplier to its dimension's base unit, plus the
// dimension it belongs to. Two units are comparable iff they share a dimension.
type unitConv struct {
	factor float64
	dim    string
}

// unitTable maps a unit string to its conversion. Base units: time→s, ratio→%,
// throughput→iops, byte-rate→MiBps. Keys are matched case-sensitively first,
// then lower-cased, so "MiBps"/"mibps" and "IOPS"/"iops" both resolve while "%"
// and "µs" keep their exact form.
var unitTable = map[string]unitConv{
	// time → seconds
	"ns": {1e-9, "time"}, "us": {1e-6, "time"}, "µs": {1e-6, "time"}, "μs": {1e-6, "time"},
	"ms": {1e-3, "time"}, "s": {1, "time"}, "sec": {1, "time"}, "secs": {1, "time"},
	"min": {60, "time"}, "mins": {60, "time"},
	"h": {3600, "time"}, "hr": {3600, "time"}, "hour": {3600, "time"}, "hours": {3600, "time"},
	// ratio → percent
	"%": {1, "ratio"}, "percent": {1, "ratio"}, "pct": {1, "ratio"}, "ratio": {100, "ratio"},
	// throughput → IOPS
	"iops": {1, "iops"},
	// byte-rate → MiBps
	"kibps": {1.0 / 1024, "byte_rate"}, "mibps": {1, "byte_rate"}, "gibps": {1024, "byte_rate"},
}

// canonicalize converts (value, unit) to its dimension's base unit. ok is false
// for an empty or unrecognized unit — callers then fall back to a raw compare so
// an unknown unit never produces a silently wrong verdict.
func canonicalize(value float64, unit string) (base float64, dim string, ok bool) {
	u := strings.TrimSpace(unit)
	if u == "" {
		return value, "", false
	}
	if c, ok := unitTable[u]; ok {
		return value * c.factor, c.dim, true
	}
	if c, ok := unitTable[strings.ToLower(u)]; ok {
		return value * c.factor, c.dim, true
	}
	return value, "", false
}

// compareWithUnits compares a measured value against an SLA bar, honoring units.
//   - Both units known and same dimension: convert to the common base, compare.
//   - Both units known but different dimensions (e.g. s vs iops): incompatible —
//     the caller emits an error rather than a bogus pass/fail.
//   - Either unit unknown/empty: raw compare (preserves pre-unit behavior).
func compareWithUnits(measured float64, measuredUnit, op string, want float64, wantUnit string) (pass, incompatible bool) {
	mv, mdim, mok := canonicalize(measured, measuredUnit)
	wv, wdim, wok := canonicalize(want, wantUnit)
	switch {
	case mok && wok && mdim == wdim:
		return compare(mv, op, wv), false
	case mok && wok && mdim != wdim:
		return false, true
	default:
		return compare(measured, op, want), false
	}
}
