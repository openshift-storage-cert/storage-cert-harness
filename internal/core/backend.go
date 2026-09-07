package core

// Backend is a named storage array configuration. Common fields are typed;
// everything vendor-specific goes in Params (arbitrary shape, may be empty).
// Credentials live in Secrets as references, never as committed plaintext.
// See decisions/0005.
type Backend struct {
	Name          string               `json:"name" yaml:"name"`
	Vendor        string               `json:"vendor,omitempty" yaml:"vendor,omitempty"`
	StorageClass  string               `json:"storage_class,omitempty" yaml:"storage_class,omitempty"`
	SnapshotClass string               `json:"snapshot_class,omitempty" yaml:"snapshot_class,omitempty"`
	Params        map[string]any       `json:"params,omitempty" yaml:"params,omitempty"`
	Secrets       map[string]SecretRef `json:"secrets,omitempty" yaml:"secrets,omitempty"`
}

// SecretRef points at a secret value without embedding it. Exactly one source
// should be set. See decisions/0005 (file / k8s / inline; no env-var refs).
type SecretRef struct {
	File      string        `json:"file,omitempty" yaml:"file,omitempty"`
	K8sSecret *K8sSecretRef `json:"k8s_secret,omitempty" yaml:"k8s_secret,omitempty"`
	Inline    string        `json:"inline,omitempty" yaml:"inline,omitempty"` // local dev only; gitignored
}

// K8sSecretRef names a key in a cluster Secret. Resolution is deferred until
// internal/kube (client-go) exists.
type K8sSecretRef struct {
	Namespace string `json:"namespace" yaml:"namespace"`
	Name      string `json:"name" yaml:"name"`
	Key       string `json:"key" yaml:"key"`
}

// ResolvedBackend is a Backend with its secrets resolved to values held only in
// memory. Secret values are unexported so they are never marshaled into a report
// or logged by accident; read them via Secret.
type ResolvedBackend struct {
	Name          string
	Vendor        string
	StorageClass  string
	SnapshotClass string
	Params        map[string]any

	secrets map[string]string
}

// NewResolvedBackend builds a ResolvedBackend from typed fields and already
// resolved secret values.
func NewResolvedBackend(b Backend, secrets map[string]string) *ResolvedBackend {
	return &ResolvedBackend{
		Name:          b.Name,
		Vendor:        b.Vendor,
		StorageClass:  b.StorageClass,
		SnapshotClass: b.SnapshotClass,
		Params:        b.Params,
		secrets:       secrets,
	}
}

// Secret returns the resolved value for key, if present.
func (r *ResolvedBackend) Secret(key string) (string, bool) {
	v, ok := r.secrets[key]
	return v, ok
}

// Param returns a vendor-specific parameter, if present.
func (r *ResolvedBackend) Param(key string) (any, bool) {
	v, ok := r.Params[key]
	return v, ok
}
