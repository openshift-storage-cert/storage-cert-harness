package virtbench

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

const (
	defaultFIOCompleteSlackSec = 360
	defaultFIOCompleteMinSec   = 420
	defaultFIOCompletePoll     = 5 * time.Second
	fioCompleteGuestPath       = "/tmp/fio_complete"
)

// fioCompleteTimeout bounds waitForFIOComplete. When fio_complete_timeout is unset
// (0), uses max(defaultFIOCompleteMinSec, fio_runtime + defaultFIOCompleteSlackSec).
func fioCompleteTimeout(tr core.TestRequirement) (time.Duration, error) {
	seconds := intParam(tr, "fio_complete_timeout", 0)
	if seconds == 0 {
		runtime := intParam(tr, "fio_runtime", defaultFIORuntime)
		seconds = runtime + defaultFIOCompleteSlackSec
		if seconds < defaultFIOCompleteMinSec {
			seconds = defaultFIOCompleteMinSec
		}
	}
	if seconds < 1 {
		return 0, fmt.Errorf("fio_complete_timeout must be >= 1")
	}
	return time.Duration(seconds) * time.Second, nil
}

func fioGuestCredentials(tr core.TestRequirement) (user, password string) {
	return strParamOr(tr, "vm_user", "cloud-user"), strParamOr(tr, "vm_password", "changeme")
}

// bashSingleQuote returns a single-quoted Bash literal safe for embedding in bash -c.
func bashSingleQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// fioGuestSSHRemoteScript builds a bash -c script for ssh-test-pod. The guest password
// is read from stdin into a temp file for sshpass -f (not placed in argv).
func fioGuestSSHRemoteScript(user, vmIP, remoteCmd string) string {
	return fmt.Sprintf(
		"set -euo pipefail; passfile=$(mktemp); chmod 600 \"$passfile\"; cat >\"$passfile\"; "+
			"trap 'rm -f \"$passfile\"' EXIT; "+
			"sshpass -f \"$passfile\" ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "+
			"-o ConnectTimeout=10 -o PreferredAuthentications=password -o PubkeyAuthentication=no "+
			"%s@%s %s",
		bashSingleQuote(user), bashSingleQuote(vmIP), bashSingleQuote(remoteCmd),
	)
}

func fioGuestFIOComplete(ctx context.Context, kubeconfig, user, password, vmIP string) (bool, error) {
	script := fioGuestSSHRemoteScript(user, vmIP, "test -f "+fioCompleteGuestPath)
	_, ok, err := runKubectlIO(ctx, kubeconfig, strings.NewReader(password), "exec", "-i",
		"-n", sshPodNS, sshPodName, "--", "/bin/bash", "-c", script)
	if err != nil {
		return false, err
	}
	return ok, nil
}

func fioVMIIPs(data []byte, wanted map[string]struct{}) (map[string]string, error) {
	var list fioVMIList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("virtbench fio: parse VM instance list: %w", err)
	}
	ips := make(map[string]string, len(wanted))
	for _, item := range list.Items {
		key := item.Metadata.Namespace + "/" + item.Metadata.Name
		if _, ok := wanted[key]; !ok {
			continue
		}
		for _, iface := range item.Status.Interfaces {
			if iface.IPAddress != "" {
				ips[item.Metadata.Namespace] = iface.IPAddress
				break
			}
		}
	}
	return ips, nil
}

func fioCompleteTimeoutErr(complete, total int, pending []string, timeout time.Duration) error {
	return fmt.Errorf("virtbench fio: %d of %d guests reported fio_complete after %s (pending: %s)",
		complete, total, timeout, strings.Join(pending, ", "))
}

// waitForFIOComplete polls each FIO guest until /tmp/fio_complete exists or timeout.
func waitForFIOComplete(ctx context.Context, rc *core.RunCtx, tr core.TestRequirement) error {
	timeout, err := fioCompleteTimeout(tr)
	if err != nil {
		return fmt.Errorf("virtbench: %s: %w", tr.ID, err)
	}
	start, end, err := fioVMRange(tr)
	if err != nil {
		return fmt.Errorf("virtbench: %s: %w", tr.ID, err)
	}
	prefix := strParamOr(tr, "namespace_prefix", "fio-latency")
	wanted := make(map[string]struct{}, end-start+1)
	namespaces := make([]string, 0, end-start+1)
	for index := start; index <= end; index++ {
		ns := fmt.Sprintf("%s-%d", prefix, index)
		namespaces = append(namespaces, ns)
		wanted[fmt.Sprintf("%s/%s", ns, defaultFIOVMName)] = struct{}{}
	}
	kubeconfig := strParamOr(tr, "kubeconfig", "")
	deadline := time.Now().Add(timeout)
	boundCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	if !sshpassReady(boundCtx, kubeconfig) {
		if err := boundCtx.Err(); err != nil {
			return fmt.Errorf("virtbench fio: ssh helper pod %s/%s not ready before timeout: %w", sshPodNS, sshPodName, err)
		}
		return fmt.Errorf("virtbench fio: ssh helper pod %s/%s is not ready for guest SSH", sshPodNS, sshPodName)
	}
	user, password := fioGuestCredentials(tr)

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("virtbench fio: timed out before listing VM instances after %s", timeout)
		}
		remaining := time.Until(deadline)
		pollCtx, pollCancel := context.WithTimeout(boundCtx, remaining)
		out, ok, runErr := runKubectl(pollCtx, kubeconfig, "get", "vmi", "--all-namespaces", "-o", "json")
		pollCancel()
		if runErr != nil {
			return fmt.Errorf("virtbench fio: list VM instances for fio_complete: %w", runErr)
		}
		if !ok {
			return fmt.Errorf("virtbench fio: list VM instances for fio_complete: %s", strings.TrimSpace(out))
		}
		ips, err := fioVMIIPs([]byte(out), wanted)
		if err != nil {
			return err
		}

		complete := 0
		var pending []string
		for _, ns := range namespaces {
			ip := ips[ns]
			if ip == "" {
				pending = append(pending, ns+"(no IP)")
				continue
			}
			remaining = time.Until(deadline)
			if remaining <= 0 {
				return fioCompleteTimeoutErr(complete, len(namespaces), pending, timeout)
			}
			checkCtx, checkCancel := context.WithTimeout(boundCtx, remaining)
			done, checkErr := fioGuestFIOComplete(checkCtx, kubeconfig, user, password, ip)
			checkCancel()
			if checkErr != nil {
				return fmt.Errorf("virtbench fio: check fio_complete on %s: %w", ns, checkErr)
			}
			if done {
				complete++
			} else {
				pending = append(pending, ns)
			}
		}
		if complete == len(namespaces) {
			rc.Logger.Info("virtbench fio: all guests reported fio_complete", "vms", complete, "timeout", timeout)
			return nil
		}
		if time.Now().After(deadline) {
			return fioCompleteTimeoutErr(complete, len(namespaces), pending, timeout)
		}
		rc.Logger.Info("virtbench fio: waiting for guest fio_complete",
			"complete", complete, "total", len(namespaces), "timeout", timeout, "pending", strings.Join(pending, ", "))
		wait := defaultFIOCompletePoll
		if remaining := time.Until(deadline); remaining < wait {
			wait = remaining
		}
		if wait <= 0 {
			return fioCompleteTimeoutErr(complete, len(namespaces), pending, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
}
