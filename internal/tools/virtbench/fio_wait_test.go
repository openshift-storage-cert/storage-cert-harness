package virtbench

import (
	"strings"
	"testing"
)

func TestBashSingleQuote(t *testing.T) {
	cases := map[string]string{
		"":            "''",
		"plain":       "'plain'",
		"a'b":         "'a'\"'\"'b'",
		"$(rm -rf /)": "'$(rm -rf /)'",
	}
	for in, want := range cases {
		if got := bashSingleQuote(in); got != want {
			t.Errorf("bashSingleQuote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFioGuestSSHRemoteScriptDoesNotEmbedPassword(t *testing.T) {
	script := fioGuestSSHRemoteScript("cloud-user", "10.0.0.5", "test -f /tmp/fio_complete")
	if strings.Contains(script, "sshpass -p") {
		t.Fatalf("password must not appear on sshpass command line:\n%s", script)
	}
	if !strings.Contains(script, "sshpass -f") || !strings.Contains(script, "cat >\"$passfile\"") {
		t.Fatalf("expected password-via-file pattern:\n%s", script)
	}
	if !strings.Contains(script, "'cloud-user'@'10.0.0.5'") {
		t.Fatalf("user and IP should be single-quoted:\n%s", script)
	}
}
