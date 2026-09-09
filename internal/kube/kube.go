// Package kube will hold the shared Kubernetes client helpers (client-go) used
// by image-based Runners and Provisioners. It is a stub today — the harness
// skeleton runs the example tool with no live cluster. See decisions/0003.
package kube

// Client is a placeholder for the shared cluster client. Provisioners and
// Runners will accept this (via core.RunCtx) once implemented.
type Client struct {
	// TODO: wrap client-go (kubernetes.Interface, dynamic client, rest.Config).
}
