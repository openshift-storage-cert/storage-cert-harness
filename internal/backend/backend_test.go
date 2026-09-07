package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadResolveFileAndInline(t *testing.T) {
	secretFile := writeTemp(t, "pw", "s3cr3t\n") // trailing newline should be trimmed
	set, err := Load(writeTemp(t, "backends.yaml", `
schema_version: "1"
backends:
  - name: array-a
    vendor: netapp-trident
    storage_class: trident-csi
    params:
      svm: svm0
    secrets:
      password:
        file: `+secretFile+`
      username:
        inline: admin
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	b, ok := set.Get("array-a")
	if !ok {
		t.Fatal("backend array-a not found")
	}
	rb, err := Resolve(b, nil, nil)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if v, _ := rb.Secret("password"); v != "s3cr3t" {
		t.Fatalf("password = %q, want s3cr3t (trimmed)", v)
	}
	if v, _ := rb.Secret("username"); v != "admin" {
		t.Fatalf("username = %q, want admin", v)
	}
	if v, _ := rb.Param("svm"); v != "svm0" {
		t.Fatalf("param svm = %v, want svm0", v)
	}
}

func TestK8sSecretRequiresReader(t *testing.T) {
	set, err := Load(writeTemp(t, "backends.yaml", `
schema_version: "1"
backends:
  - name: array-k8s
    secrets:
      token:
        k8s_secret:
          namespace: ns
          name: creds
          key: token
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	b, _ := set.Get("array-k8s")
	if _, err := Resolve(b, nil, nil); err == nil || !strings.Contains(err.Error(), "not yet wired") {
		t.Fatalf("expected not-yet-wired error, got %v", err)
	}
}

func TestValidateRejectsBadRefs(t *testing.T) {
	cases := map[string]string{
		"no source": `schema_version: "1"
backends:
  - name: a
    secrets:
      x: {}`,
		"two sources": `schema_version: "1"
backends:
  - name: a
    secrets:
      x: {file: /f, inline: v}`,
		"dup name": `schema_version: "1"
backends:
  - name: a
  - name: a`,
		"bad schema": `schema_version: "9"
backends: []`,
	}
	for name, body := range cases {
		if _, err := Load(writeTemp(t, "b.yaml", body)); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}
