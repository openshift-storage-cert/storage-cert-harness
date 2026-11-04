package kubeburnerocp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
	"github.com/openshift-storage-cert/storage-cert-harness/internal/grader"
)

type virtParallelEvaluator struct{}

func (virtParallelEvaluator) Evaluate(_ context.Context, tr core.TestRequirement, res core.TestResult) ([]core.Verdict, error) {
	var raw struct {
		Lifecycle *virtParallelEvidence `json:"lifecycle"`
		Error     string                `json:"error"`
	}
	outcome := res.Checks[virtParallelCheck]
	reason := "missing lifecycle evidence"
	expected, actual := "", ""
	if json.Unmarshal(res.Raw, &raw) != nil || raw.Lifecycle == nil {
		outcome = core.OutcomeError
		if raw.Error != "" {
			reason = raw.Error
		}
	} else {
		e := raw.Lifecycle
		reason = e.Reason
		expected = fmt.Sprintf("%d distinct VM migrations", e.Expected)
		actual = fmt.Sprintf("%d distinct VM migrations", e.Completed)
		p, err := virtParallelParams(tr.Params)
		if err != nil {
			outcome = core.OutcomeError
			reason = err.Error()
		} else {
			initial, _ := asInt(p["initial_vms"])
			increment, _ := asInt(p["increment"])
			iterations, _ := asInt(p["max_iterations"])
			total, totalErr := virtParallelTotal(initial, increment, iterations)
			if totalErr != nil {
				return nil, totalErr
			}
			expected = fmt.Sprintf("%d distinct VM migrations", total)
			if initial != e.InitialVMs || increment != e.Increment || iterations != e.Iterations {
				outcome = core.OutcomeError
				reason = "lifecycle workload size does not match the effective plan/catalog parameters"
			}
		}
	}
	vs := grader.GradeSLAs(tr, res)
	checks := grader.GradeChecks(tr, res)
	found := false
	for i := range checks {
		if checks[i].Item == "check:"+virtParallelCheck {
			checks[i].Outcome = outcome
			checks[i].Reason = reason
			checks[i].Expected = expected
			checks[i].Actual = actual
			found = true
		}
	}
	vs = append(vs, checks...)
	if !found {
		vs = append(vs, core.Verdict{TR: tr.ID, Item: "check:" + virtParallelCheck, Outcome: outcome, Reason: reason, Expected: expected, Actual: actual})
	}
	return vs, nil
}
