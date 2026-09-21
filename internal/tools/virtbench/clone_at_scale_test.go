package virtbench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
)

// TestCloneAtScaleArgsForcesTwoDisks guards the fix itself: --num-disks 2 and
// a staged --vm-template are always present, and any num_disks/vm_template a
// plan sets are ignored — TR-VIRT-018's disk count is fixed, not configurable.
func TestCloneAtScaleArgsForcesTwoDisks(t *testing.T) {
	root := t.TempDir()
	tr := core.TestRequirement{
		ID: "TR-VIRT-018",
		Params: map[string]any{
			"storage_class":    "ocs-storagecluster-ceph-rbd",
			"iteration_clones": 5,
			"num_disks":        1,                   // must be ignored
			"vm_template":      "/should/be/unused", // must be ignored
		},
	}
	args, err := cloneAtScaleArgs("virtbench")(&core.RunCtx{}, tr, root)
	if err != nil {
		t.Fatalf("cloneAtScaleArgs: %v", err)
	}

	var staged string
	for i, a := range args {
		switch a {
		case "--num-disks":
			if i+1 >= len(args) || args[i+1] != "2" {
				t.Fatalf("--num-disks not forced to 2; args = %v", args)
			}
		case "--vm-template":
			if i+1 < len(args) {
				staged = args[i+1]
			}
		}
	}
	if staged == "" {
		t.Fatalf("--vm-template not passed; args = %v", args)
	}
	if filepath.Dir(staged) != root {
		t.Errorf("template staged at %q, want inside results root %q", staged, root)
	}
	if staged == "/should/be/unused" {
		t.Error("the plan's vm_template param leaked into the CLI; it must be ignored")
	}
}

// TestCloneAtScaleArgsStagedTemplateHasTwoDataVolumes confirms the actual bug
// fix: unlike virtbench's --num-disks flag (which only labels output), the
// staged template really does define two DataVolumes.
func TestCloneAtScaleArgsStagedTemplateHasTwoDataVolumes(t *testing.T) {
	root := t.TempDir()
	tr := core.TestRequirement{ID: "TR-VIRT-018", Params: map[string]any{"storage_class": "sc"}}
	args, err := cloneAtScaleArgs("virtbench")(&core.RunCtx{}, tr, root)
	if err != nil {
		t.Fatalf("cloneAtScaleArgs: %v", err)
	}
	var staged string
	for i, a := range args {
		if a == "--vm-template" && i+1 < len(args) {
			staged = args[i+1]
		}
	}
	if staged == "" {
		t.Fatal("--vm-template not passed")
	}
	data, err := os.ReadFile(staged)
	if err != nil {
		t.Fatalf("read staged template: %v", err)
	}

	// virtbench substitutes {{STORAGE_CLASS_NAME}} at run time; unresolved, it
	// isn't valid YAML, so neutralize it before parsing (this test runs before
	// virtbench ever touches the file).
	clean := strings.ReplaceAll(string(data), "{{STORAGE_CLASS_NAME}}", "placeholder-sc")

	var doc struct {
		Spec struct {
			DataVolumeTemplates []map[string]any `yaml:"dataVolumeTemplates"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal([]byte(clean), &doc); err != nil {
		t.Fatalf("parse staged template: %v", err)
	}
	if got := len(doc.Spec.DataVolumeTemplates); got != 2 {
		t.Errorf("staged template has %d dataVolumeTemplates, want 2", got)
	}
}
