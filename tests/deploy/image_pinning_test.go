// Package deploy_test guards the deployment manifests and Dockerfiles
// against regressions of Aikido findings that flag unpinned base images
// (finding #123 — "Automatic upgrades of base Docker images can lead to
// supply chain attacks"). These are plain file-content checks so they
// run in `make test` without Docker, kubectl, or kustomize.
package deploy_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// repoRoot returns the repository root relative to this test file
// (tests/deploy/). The checks intentionally use filesystem paths so the
// test fails loudly if a manifest is moved or renamed.
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	// tests/deploy -> repo root
	return filepath.Clean(filepath.Join(wd, "..", ".."))
}

// TestK8sBaseImages_DoNotUseLatestTag verifies the checked-in Kubernetes
// deployment manifests never reference `:latest`. `:latest` is flagged by
// Aikido #123 because it silently pulls whatever digest happens to be
// current, defeating supply-chain pinning.
func TestK8sBaseImages_DoNotUseLatestTag(t *testing.T) {
	root := repoRoot(t)
	manifestDirs := []string{
		filepath.Join(root, "deploy", "k8s", "base"),
		filepath.Join(root, "deploy", "k8s", "overlays", "staging"),
		filepath.Join(root, "deploy", "k8s", "overlays", "production"),
	}

	// image: <ref>:latest (optionally followed by a digest) is disallowed.
	// The regex anchors on the `image:` key to avoid false-positives in
	// comments or kube-linter allow annotations.
	latestRE := regexp.MustCompile(`(?m)^\s*image:\s*\S+:latest(\s|$)`)

	for _, dir := range manifestDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			if loc := latestRE.FindIndex(data); loc != nil {
				t.Errorf("%s uses an unpinned :latest image tag — Aikido #123 "+
					"regression. Replace with an explicit version or @sha256 digest. "+
					"Offending line: %q", path, strings.TrimSpace(string(data[loc[0]:loc[1]])))
			}
		}
	}
}

// TestDockerfileBaseImages_ArePinnedByDigest verifies each Dockerfile
// pins its FROM base images by @sha256 digest. A dangling tag like
// `golang:1.26-alpine` is the exact class of dependency Aikido #123
// warns about.
func TestDockerfileBaseImages_ArePinnedByDigest(t *testing.T) {
	root := repoRoot(t)
	dockerfiles := []string{
		filepath.Join(root, "docker", "aixgo.Dockerfile"),
		filepath.Join(root, "docker", "ollama.Dockerfile"),
	}

	// Match any `FROM <ref>` line, then require it to contain @sha256:<hex>.
	fromRE := regexp.MustCompile(`(?mi)^FROM\s+(\S+)`)
	digestRE := regexp.MustCompile(`@sha256:[0-9a-f]{64}`)

	for _, path := range dockerfiles {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, m := range fromRE.FindAllSubmatch(data, -1) {
			ref := string(m[1])
			// `FROM scratch` has no upstream image to pin.
			if ref == "scratch" {
				continue
			}
			if !digestRE.MatchString(ref) {
				t.Errorf("%s: FROM %s is not pinned by @sha256 digest "+
					"(Aikido #123 regression). Re-pin with "+
					"`docker buildx imagetools inspect <ref>` to fetch the digest.", path, ref)
			}
		}
	}
}
