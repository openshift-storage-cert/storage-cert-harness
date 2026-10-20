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

func TestValidateGuestSSHUser(t *testing.T) {
	for _, user := range []string{"cloud-user", "fedora", "a", "a_b-c"} {
		if err := validateGuestSSHUser(user); err != nil {
			t.Errorf("valid user %q: %v", user, err)
		}
	}
	for _, user := range []string{"", "-root", "Root", "-oProxyCommand=id", "user@host"} {
		if err := validateGuestSSHUser(user); err == nil {
			t.Errorf("invalid user %q should be rejected", user)
		}
	}
}

func TestValidateGuestSSHIP(t *testing.T) {
	if err := validateGuestSSHIP("10.0.0.5"); err != nil {
		t.Fatal(err)
	}
	if err := validateGuestSSHIP("not-an-ip"); err == nil {
		t.Fatal("expected invalid IP error")
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
	for _, want := range []string{
		"StrictHostKeyChecking=yes",
		"ssh-keyscan",
		"-l 'cloud-user'",
		"-- '10.0.0.5'",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q:\n%s", want, script)
		}
	}
	if strings.Contains(script, "'cloud-user'@") {
		t.Fatalf("user@host form must not be used (SSH option injection):\n%s", script)
	}
}
