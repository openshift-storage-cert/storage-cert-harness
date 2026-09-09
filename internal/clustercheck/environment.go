package clustercheck

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"gitlab.cee.redhat.com/eco-special-projects/storage-cert-harness/internal/core"
)

// CollectEnvironment gathers the run environment (hardware + platform) from the
// cluster, best-effort: any datum that cannot be read is simply omitted. cli is
// "oc" or "kubectl" (see KubeCLI). scUnderTest is the StorageClass being tested
// (from the active backend); its provisioner determines the reported CSI driver.
// When empty, the cluster's default StorageClass is used. See ADR-0014.
func CollectEnvironment(ctx context.Context, cli, scUnderTest string) core.Environment {
	return core.Environment{
		Hardware: collectHardware(ctx, cli),
		Platform: collectPlatform(ctx, cli, scUnderTest),
	}
}

func collectHardware(ctx context.Context, cli string) core.Hardware {
	hw := core.Hardware{}
	var nl struct {
		Items []struct {
			Status struct {
				Capacity map[string]string `json:"capacity"`
				NodeInfo struct {
					Architecture string `json:"architecture"`
					OSImage      string `json:"osImage"`
				} `json:"nodeInfo"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := runJSON(ctx, cli, &nl, "get", "nodes", "-o", "json"); err != nil {
		return hw
	}
	hw.Nodes = len(nl.Items)
	if len(nl.Items) > 0 {
		n := nl.Items[0]
		const src = "oc get nodes -o json"
		hw.Facts = facts(src,
			"arch", n.Status.NodeInfo.Architecture,
			"os_image", n.Status.NodeInfo.OSImage,
			"cpu_per_node", n.Status.Capacity["cpu"],
			"ram_per_node", kiToGi(n.Status.Capacity["memory"]),
		)
	}
	return hw
}

func collectPlatform(ctx context.Context, cli, scUnderTest string) core.Platform {
	p := core.Platform{}

	p.OCPVersion = strings.TrimSpace(runText(ctx, cli,
		"get", "clusterversion", "version", "-o", "jsonpath={.status.desired.version}"))

	var ver struct {
		ServerVersion struct {
			GitVersion string `json:"gitVersion"`
		} `json:"serverVersion"`
	}
	if runJSON(ctx, cli, &ver, "version", "-o", "json") == nil {
		p.KubeVersion = ver.ServerVersion.GitVersion
	}

	// Operators (CSVs) in openshift-cnv; also yields the CNV version.
	var csvs struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				Version string `json:"version"`
			} `json:"spec"`
		} `json:"items"`
	}
	if runJSON(ctx, cli, &csvs, "get", "csv", "-n", "openshift-cnv", "-o", "json") == nil {
		for _, c := range csvs.Items {
			base := strings.SplitN(c.Metadata.Name, ".v", 2)[0]
			p.Operators = append(p.Operators, core.OperatorInfo{
				Name: base, Version: c.Spec.Version, Source: "oc get csv -n openshift-cnv",
			})
			if strings.Contains(base, "kubevirt-hyperconverged") {
				p.CNVVersion = c.Spec.Version
			}
		}
	}

	// StorageClasses; remember the default provisioner for the CSIDriver lookup.
	var scs struct {
		Items []struct {
			Metadata struct {
				Name        string            `json:"name"`
				Annotations map[string]string `json:"annotations"`
			} `json:"metadata"`
			Provisioner       string            `json:"provisioner"`
			ReclaimPolicy     string            `json:"reclaimPolicy"`
			VolumeBindingMode string            `json:"volumeBindingMode"`
			Parameters        map[string]string `json:"parameters"`
		} `json:"items"`
	}
	defaultProvisioner := ""
	testedProvisioner := ""
	if runJSON(ctx, cli, &scs, "get", "sc", "-o", "json") == nil {
		for _, s := range scs.Items {
			isDefault := s.Metadata.Annotations["storageclass.kubernetes.io/is-default-class"] == "true"
			if isDefault {
				defaultProvisioner = s.Provisioner
			}
			if s.Metadata.Name == scUnderTest {
				testedProvisioner = s.Provisioner
			}
			p.StorageClasses = append(p.StorageClasses, core.StorageClassInfo{
				Name:              s.Metadata.Name,
				Provisioner:       s.Provisioner,
				IsDefault:         isDefault,
				ReclaimPolicy:     s.ReclaimPolicy,
				VolumeBindingMode: s.VolumeBindingMode,
				Parameters:        s.Parameters,
			})
		}
	}

	// The driver under test is the provisioner of the StorageClass under test;
	// fall back to the cluster default when no backend SC was given (ADR-0014).
	provisioner := testedProvisioner
	if provisioner == "" {
		provisioner = defaultProvisioner
	}
	if provisioner != "" {
		p.CSIDriver = collectCSIDriver(ctx, cli, provisioner)
	}
	return p
}

func collectCSIDriver(ctx context.Context, cli, name string) core.CSIDriverInfo {
	info := core.CSIDriverInfo{Name: name}
	var d struct {
		Spec struct {
			AttachRequired       *bool    `json:"attachRequired"`
			FSGroupPolicy        string   `json:"fsGroupPolicy"`
			VolumeLifecycleModes []string `json:"volumeLifecycleModes"`
		} `json:"spec"`
	}
	if runJSON(ctx, cli, &d, "get", "csidriver", name, "-o", "json") == nil {
		if d.Spec.AttachRequired != nil {
			info.AttachRequired = *d.Spec.AttachRequired
		}
		info.FSGroupPolicy = d.Spec.FSGroupPolicy
		info.VolumeLifecycleModes = d.Spec.VolumeLifecycleModes
	}
	return info
}

// ---- small helpers ----

func runText(ctx context.Context, cli string, args ...string) string {
	out, err := exec.CommandContext(ctx, cli, args...).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func runJSON(ctx context.Context, cli string, v any, args ...string) error {
	out, err := exec.CommandContext(ctx, cli, args...).Output()
	if err != nil {
		return err
	}
	return json.Unmarshal(out, v)
}

// facts builds a source-tagged Fact slice from key,value pairs, dropping empties.
func facts(source string, kv ...string) []core.Fact {
	var out []core.Fact
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] == "" {
			continue
		}
		out = append(out, core.Fact{Key: kv[i], Value: kv[i+1], Source: source})
	}
	return out
}

func kiToGi(s string) string {
	s = strings.TrimSuffix(s, "Ki")
	ki, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%.0fGi", ki/1024/1024)
}
