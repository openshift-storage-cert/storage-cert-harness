package orchestrator

import (
	"log/slog"
	"sync"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
)

// progressTracker reports plan execution progress at the Test Requirement
// execution level. Named variants are separate executions, while a TR's
// individual SLA/check verdicts are intentionally not counted separately.
type progressTracker struct {
	logger *slog.Logger

	mu       sync.Mutex
	order    []string
	state    map[string]progressState
	outcomes map[string]core.Outcome
}

type progressState uint8

const (
	progressQueued progressState = iota
	progressRunning
	progressComplete
)

func newProgressTracker(logger *slog.Logger, trs []core.TestRequirement) *progressTracker {
	p := &progressTracker{
		logger:   logger,
		order:    make([]string, 0, len(trs)),
		state:    make(map[string]progressState, len(trs)),
		outcomes: make(map[string]core.Outcome, len(trs)),
	}
	for _, tr := range trs {
		label := core.ExecutionLabel(tr.ID, tr.Variant)
		p.order = append(p.order, label)
		p.state[label] = progressQueued
	}
	p.loggerInfo("plan progress",
		"status", "started",
		"completed", 0,
		"total", len(p.order),
		"remaining", len(p.order),
		"queued", append([]string(nil), p.order...),
	)
	return p
}

func (p *progressTracker) start(tr core.TestRequirement) {
	if p == nil {
		return
	}
	label := core.ExecutionLabel(tr.ID, tr.Variant)
	p.mu.Lock()
	if p.state[label] != progressQueued {
		p.mu.Unlock()
		return
	}
	p.state[label] = progressRunning
	completed, total := p.countsLocked()
	running, queued := p.listsLocked(progressRunning, progressQueued)
	p.loggerDebug("plan progress detail",
		"status", "running",
		"test", label,
		"completed", completed,
		"total", total,
		"remaining", total-completed,
		"running", running,
		"queued", queued,
	)
	p.mu.Unlock()
}

func (p *progressTracker) complete(tr core.TestRequirement, outcome core.Outcome) {
	if p == nil {
		return
	}
	label := core.ExecutionLabel(tr.ID, tr.Variant)
	p.mu.Lock()
	if p.state[label] == progressComplete {
		p.mu.Unlock()
		return
	}
	p.state[label] = progressComplete
	p.outcomes[label] = outcome
	completed, total := p.countsLocked()
	running, queued := p.listsLocked(progressRunning, progressQueued)
	passed, failed, errors, skipped := p.outcomeListsLocked()
	p.loggerInfo("test progress",
		"status", "completed",
		"test", label,
		"outcome", outcome,
		"completed", completed,
		"total", total,
		"remaining", total-completed,
	)
	p.loggerDebug("plan progress detail",
		"status", "updated",
		"running", running,
		"queued", queued,
	)
	if completed == total {
		p.loggerInfo("plan progress",
			"status", "complete",
			"completed", completed,
			"total", total,
			"remaining", 0,
			"passed", passed,
			"failed", failed,
			"errors", errors,
			"skipped", skipped,
			"queued", queued,
		)
	}
	p.mu.Unlock()
}

func (p *progressTracker) countsLocked() (completed, total int) {
	total = len(p.order)
	for _, label := range p.order {
		if p.state[label] == progressComplete {
			completed++
		}
	}
	return completed, total
}

func (p *progressTracker) listsLocked(wanted ...progressState) (running, queued []string) {
	for _, label := range p.order {
		switch p.state[label] {
		case progressRunning:
			for _, state := range wanted {
				if state == progressRunning {
					running = append(running, label)
					break
				}
			}
		case progressQueued:
			for _, state := range wanted {
				if state == progressQueued {
					queued = append(queued, label)
					break
				}
			}
		}
	}
	return running, queued
}

func (p *progressTracker) outcomeListsLocked() (passed, failed, errors, skipped []string) {
	for _, label := range p.order {
		switch p.outcomes[label] {
		case core.OutcomePass:
			passed = append(passed, label)
		case core.OutcomeFail:
			failed = append(failed, label)
		case core.OutcomeError:
			errors = append(errors, label)
		case core.OutcomeSkip:
			skipped = append(skipped, label)
		}
	}
	return passed, failed, errors, skipped
}

func (p *progressTracker) loggerInfo(msg string, args ...any) {
	if p != nil && p.logger != nil {
		p.logger.Info(msg, args...)
	}
}

func (p *progressTracker) loggerDebug(msg string, args ...any) {
	if p != nil && p.logger != nil {
		p.logger.Debug(msg, args...)
	}
}

// summarizeTR collapses the item-level verdicts for one execution into the
// user-facing progress outcome. Errors take precedence over failures, then
// passes; a result containing only skips is shown as skipped.
func summarizeTR(verdicts []core.Verdict, tr core.TestRequirement) core.Outcome {
	best := core.OutcomeSkip
	seen := false
	for _, verdict := range verdicts {
		if verdict.TR != tr.ID || verdict.Variant != tr.Variant {
			continue
		}
		seen = true
		if outcomeRank(verdict.Outcome) > outcomeRank(best) {
			best = verdict.Outcome
		}
	}
	if !seen {
		return core.OutcomeError
	}
	return best
}

func outcomeRank(outcome core.Outcome) int {
	switch outcome {
	case core.OutcomeError:
		return 3
	case core.OutcomeFail:
		return 2
	case core.OutcomePass:
		return 1
	default:
		return 0
	}
}
