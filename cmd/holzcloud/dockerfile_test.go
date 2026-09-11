package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The image is the one deployment where loopback is never the right address.
//
// Since 1.10 the process binds 127.0.0.1 unless HOLZCLOUD_LISTEN says
// otherwise, which is right for the documented host install with Caddy beside
// it. Inside a container it is wrong without exception: the container is its
// own network namespace, and a published port, a Kubernetes Service and the
// kubelet's probes all arrive on the pod's address and find nobody listening.
// The container starts, logs nothing wrong and answers no request.
//
// DEPLOY.md told operators to set the variable. The cluster this project runs
// on did not, and Renovate merges a new image of this repository without a
// review — so the release would have taken the site down on its own.
//
// Only the runtime stage counts: an ENV in the build stage never reaches the
// image that runs.
func TestTheImageListensOnItsOwnNamespace(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "Dockerfile"))
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	env := runtimeEnv(string(b))

	if got, ok := env["HOLZCLOUD_LISTEN"]; !ok || got != "0.0.0.0" {
		t.Errorf("the runtime stage sets HOLZCLOUD_LISTEN=%q (present: %v), want 0.0.0.0 — "+
			"without it the container binds loopback inside its own namespace and answers nobody", got, ok)
	}
	// The control: the stage parser does find the settings that are there, or
	// a parser that finds nothing would pass the check above by failing it for
	// the wrong reason and the check below would say so.
	if got := env["HOLZCLOUD_PORT"]; got != "8080" {
		t.Errorf("the runtime stage sets HOLZCLOUD_PORT=%q, want 8080 — the port the image exposes", got)
	}
}

// runtimeEnv joins continued lines and collects the KEY=VALUE pairs of every
// ENV instruction after the last FROM.
func runtimeEnv(dockerfile string) map[string]string {
	joined := strings.ReplaceAll(dockerfile, "\\\n", " ")
	var stage []string
	for _, line := range strings.Split(joined, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToUpper(trimmed), "FROM ") {
			stage = nil
			continue
		}
		stage = append(stage, trimmed)
	}
	env := map[string]string{}
	for _, line := range stage {
		if !strings.HasPrefix(strings.ToUpper(line), "ENV ") {
			continue
		}
		for _, field := range strings.Fields(line[len("ENV "):]) {
			if key, value, ok := strings.Cut(field, "="); ok {
				env[key] = strings.Trim(value, `"`)
			}
		}
	}
	return env
}
