// Package kubeburner drives upstream kube-burner (generic init -c) to measure VM-snapshot creation time for TR-VIRT-010. See decisions/0012.
package kubeburner

import (
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/registry"
	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/stages"
)

// DefaultImage is the published kube-burner image (ADR-0003), pinned to a v2.x tag; overridable via KUBE_BURNER_IMAGE. See decisions/0012 for the pin rationale.
const DefaultImage = "quay.io/kube-burner/kube-burner:v2.8.1"

func init() {
	registry.Register(stages.ToolIntegration{
		Name:              "kube-burner",
		Image:             DefaultImage,
		Provides:          []string{"TR-VIRT-010"},
		Preflight:         preflight{},
		Provisioner:       provisioner{},
		Runner:            runner{},
		LogCollector:      collector{},
		ResultParser:      parser{},
		Teardown:          teardown{},
		Evaluators:        map[string]stages.Evaluator{"TR-VIRT-010": snapshotEvaluator{}},
		ParallelSafe:      false,
		ExclusivityGroups: []string{"kube-burner", "cdi", "storage"},
	})
}
