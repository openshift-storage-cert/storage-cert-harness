package kubeburnerocp

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/openshift-storage-cert/storage-cert-harness/internal/core"
)

func TestLogSubprocessFailure(t *testing.T) {
	const limit = 32 * 1024

	tests := []struct {
		name       string
		stdout     string
		stderr     string
		wantOutput []string
		wantAbsent []string
	}{
		{
			name:       "missing output",
			wantAbsent: []string{"stream=stdout", "stream=stderr"},
		},
		{
			name:       "stderr ascii",
			stderr:     "hook failed: expected 20 migrations, got 0",
			wantOutput: []string{"stream=stderr", "hook failed: expected 20 migrations, got 0"},
			wantAbsent: []string{"stream=stdout"},
		},
		{
			name:       "stderr unicode",
			stderr:     "migration hook failed: 成功 ✅ — проверка не удалась",
			wantOutput: []string{"stream=stderr", "成功 ✅ — проверка не удалась"},
		},
		{
			name:       "stdout and stderr",
			stdout:     "preparing migration baseline",
			stderr:     "baseline file is unavailable",
			wantOutput: []string{"stream=stdout", "preparing migration baseline", "stream=stderr", "baseline file is unavailable"},
		},
		{
			name:       "large output is truncated to tail",
			stderr:     strings.Repeat("x", limit) + "TAIL",
			wantOutput: []string{"[truncated to last 32 KiB]", "TAIL"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var log bytes.Buffer
			rc := &core.RunCtx{Logger: slog.New(slog.NewTextHandler(&log, nil))}
			logSubprocessFailure(rc, kboRun{
				TRID:   "TR-VIRT-019",
				stdout: bytes.NewBufferString(tt.stdout),
				stderr: bytes.NewBufferString(tt.stderr),
			})
			got := log.String()
			for _, want := range tt.wantOutput {
				if !strings.Contains(got, want) {
					t.Errorf("log missing %q; got %q", want, got)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("log unexpectedly contains %q; got %q", absent, got)
				}
			}
		})
	}
}
