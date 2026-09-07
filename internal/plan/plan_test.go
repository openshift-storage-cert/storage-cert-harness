package plan

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "plan.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadAndFilter(t *testing.T) {
	p, err := Load(writeTemp(t, `
schema_version: "1"
name: perf
backend: array-a
concurrency: 3
select:
  partner_levels: [3]
overrides:
  TR-1:
    iodepth: 32
scheduling:
  example:
    parallel_safe: false
    exclusivity_groups: [array-a]
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if p.Backend != "array-a" || p.Concurrency != 3 {
		t.Fatalf("unexpected plan header: %+v", p)
	}
	f := p.Filter()
	if len(f.PartnerLevels) != 1 || f.PartnerLevels[0] != 3 {
		t.Fatalf("unexpected filter: %+v", f)
	}
	if got := p.Overrides["TR-1"]["iodepth"]; got != 32 {
		t.Fatalf("override iodepth = %v, want 32", got)
	}
	sched := p.SchedulingOverrides()
	s, ok := sched["example"]
	if !ok || s.ParallelSafe == nil || *s.ParallelSafe != false ||
		len(s.ExclusivityGroups) != 1 || s.ExclusivityGroups[0] != "array-a" {
		t.Fatalf("unexpected scheduling override: %+v", sched)
	}
}

func TestSelectAllYieldsEmptyFilter(t *testing.T) {
	p, err := Load(writeTemp(t, `
schema_version: "1"
name: full
select:
  all: true
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	f := p.Filter()
	if len(f.PartnerLevels)+len(f.Tools)+len(f.IDs) != 0 {
		t.Fatalf("all: true should yield an empty filter, got %+v", f)
	}
}

func TestValidateErrors(t *testing.T) {
	cases := map[string]string{
		"bad schema": `schema_version: "9"
name: x
select: {all: true}`,
		"empty select": `schema_version: "1"
name: x
select: {}`,
		"missing name": `schema_version: "1"
select: {all: true}`,
	}
	for name, body := range cases {
		if _, err := Load(writeTemp(t, body)); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}
