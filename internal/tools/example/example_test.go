package example

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

// update regenerates the golden file: go test ./... -update
var update = flag.Bool("update", false, "update golden files")

func TestRunnerUsesDistinctHandlesWithinRun(t *testing.T) {
	rc := &core.RunCtx{RunID: "same-run", Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	handles := map[string]bool{}
	for i := 0; i < 2; i++ {
		bag := core.NewBag()
		h, err := (runner{}).Run(context.Background(), rc, bag, nil)
		if err != nil {
			t.Fatal(err)
		}
		if h.ID == "" || handles[h.ID] {
			t.Fatalf("expected distinct nonempty handle, got %q", h.ID)
		}
		handles[h.ID] = true
		job, ok := core.GetAs[string](bag, "job")
		if !ok || job != h.ID {
			t.Fatalf("bag job = %q, want %q", job, h.ID)
		}
	}
}

// TestParseResults is the golden-file pattern every tool's ResultParser test
// should follow: feed a fixture, parse, compare normalized output to a golden
// file. No cluster required — this is what makes parser work a small, parallel,
// independently-ownable task (see decisions/0003).
func TestParseResults(t *testing.T) {
	in, err := os.ReadFile(filepath.Join("testdata", "sample-results.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	got, err := ParseResults(in)
	if err != nil {
		t.Fatalf("ParseResults: %v", err)
	}
	gotJSON, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	goldenPath := filepath.Join("testdata", "parse.golden.json")
	if *update {
		if err := os.WriteFile(goldenPath, append(gotJSON, '\n'), 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if strings.TrimSpace(string(gotJSON)) != strings.TrimSpace(string(want)) {
		t.Errorf("parsed output differs from golden:\n got: %s\nwant: %s", gotJSON, want)
	}
}

func TestSyntheticResultsGolden(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "synthetic-results.json"))
	if err != nil {
		t.Fatal(err)
	}
	results, err := ParseResults(data)
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "synthetic-parse.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != strings.TrimSpace(string(want)) {
		t.Fatalf("synthetic parse differs from golden: %s", got)
	}
}

func TestSyntheticRunnerRejectsInvalidValues(t *testing.T) {
	rc := &core.RunCtx{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	for _, value := range []any{nil, "12", true} {
		tr := core.TestRequirement{ID: "EX-006", Params: map[string]any{"sample_value": value}}
		if _, err := (runner{}).Run(context.Background(), rc, core.NewBag(), []core.TestRequirement{tr}); err == nil || !strings.Contains(err.Error(), "EX-006 sample_value") {
			t.Errorf("value %v: got %v, want sample_value error", value, err)
		}
	}
}

func TestSyntheticRunnerEmitsConfiguredMeasurement(t *testing.T) {
	ctx := context.Background()
	rc := &core.RunCtx{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	tr := core.TestRequirement{ID: "EX-006", Params: map[string]any{"sample_value": 12}}
	bag := core.NewBag()
	handle, err := (runner{}).Run(ctx, rc, bag, []core.TestRequirement{tr})
	if err != nil {
		t.Fatal(err)
	}
	logs, err := (collector{}).Collect(ctx, rc, bag, handle)
	if err != nil {
		t.Fatal(err)
	}
	results, err := (parser{}).Parse(ctx, rc, bag, logs)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if result.TRID != tr.ID {
			continue
		}
		if len(result.Metrics) != 1 || result.Metrics[0].Name != "sample_value" || result.Metrics[0].Value != 12 || result.Metrics[0].Unit != "count" {
			t.Fatalf("unexpected synthetic measurement: %+v", result.Metrics)
		}
		return
	}
	t.Fatal("synthetic result missing")
}
