package orchestrator

import (
	"testing"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
)

func TestResumeExecutionsSkipsOnlySuccessfulMatchingScenario(t *testing.T) {
	selected := []core.TestRequirement{
		{ID: "TR-A"},
		{ID: "TR-B"},
		{ID: "TR-C"},
		{ID: "TR-D"},
	}
	prior := core.Report{Executions: []core.RunExecution{
		{TR: "TR-A", Scenario: "load80", State: core.ExecutionCompleted, Outcome: core.OutcomePass},
		{TR: "TR-B", Scenario: "load80", State: core.ExecutionCompleted, Outcome: core.OutcomeFail},
		{TR: "TR-C", Scenario: "load80", State: core.ExecutionRunning},
		{TR: "TR-D", Scenario: "non-load80", State: core.ExecutionCompleted, Outcome: core.OutcomePass},
	}}
	got := ResumeExecutions(selected, prior, "load80")
	if len(got) != 2 || got[0].ID != "TR-C" || got[1].ID != "TR-D" {
		t.Fatalf("resumed selection = %+v, want C/D", got)
	}
	got = ResumeExecutionsWithOptions(selected, prior, "load80", true)
	if len(got) != 3 || got[0].ID != "TR-B" || got[1].ID != "TR-C" || got[2].ID != "TR-D" {
		t.Fatalf("retry-failed selection = %+v, want B/C/D", got)
	}
}

func TestResumeExecutionsSupportsLegacyVerdicts(t *testing.T) {
	selected := []core.TestRequirement{{ID: "TR-A"}, {ID: "TR-B"}}
	prior := core.Report{Verdicts: []core.Verdict{
		{TR: "TR-A", Scenario: "idle", Item: "check:a", Outcome: core.OutcomePass},
		{TR: "TR-A", Scenario: "idle", Item: "check:b", Outcome: core.OutcomeSkip},
		{TR: "TR-B", Scenario: "idle", Item: "check:a", Outcome: core.OutcomeError},
	}}
	got := ResumeExecutions(selected, prior, "idle")
	if len(got) != 0 {
		t.Fatalf("legacy resumed selection = %+v, want no completed executions", got)
	}
	got = ResumeExecutionsWithOptions(selected, prior, "idle", true)
	if len(got) != 1 || got[0].ID != "TR-B" {
		t.Fatalf("legacy retry-failed selection = %+v, want B", got)
	}
}

func TestResumeExecutionsRerunsAbortedExecutionDespiteCancellationVerdicts(t *testing.T) {
	selected := []core.TestRequirement{{ID: "TR-A"}, {ID: "TR-B"}}
	prior := core.Report{
		Executions: []core.RunExecution{
			{TR: "TR-A", Scenario: "idle", State: core.ExecutionAborted, Outcome: core.OutcomeError},
			{TR: "TR-B", Scenario: "idle", State: core.ExecutionCompleted, Outcome: core.OutcomePass},
		},
		Verdicts: []core.Verdict{
			{TR: "TR-A", Scenario: "idle", Item: "check:a", Outcome: core.OutcomeError},
			{TR: "TR-B", Scenario: "idle", Item: "check:a", Outcome: core.OutcomePass},
		},
	}
	got := ResumeExecutions(selected, prior, "idle")
	if len(got) != 1 || got[0].ID != "TR-A" {
		t.Fatalf("aborted resumed selection = %+v, want TR-A", got)
	}
}
