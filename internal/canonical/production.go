package canonical

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// NewVerifier binds the production seams of a Verifier: os-backed file reads
// rooted at the tenant and home trees and the Go toolchain module resolution.
func NewVerifier(tenantRoot, homeRoot string, stdout, stderr io.Writer) Verifier {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	return Verifier{
		TenantRoot: tenantRoot,
		ReadTenant: func(path string) ([]byte, error) {
			return os.ReadFile(filepath.Join(tenantRoot, filepath.FromSlash(path)))
		},
		ReadHome: func(path string) ([]byte, error) {
			return os.ReadFile(filepath.Join(homeRoot, filepath.FromSlash(path)))
		},
		ReadModule: func(dir, path string) ([]byte, error) {
			return os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
		},
		ResolveModule: ResolveModuleDir,
		Stdout:        stdout,
		Stderr:        stderr,
	}
}

// commandOutput is the production process execution of the module resolution.
func commandOutput(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	return command.CombinedOutput()
}

// execOutput is the process seam of the module resolution.
var execOutput = commandOutput

// ResolveModuleDir resolves a module's cache directory within a tooling
// module directory through the Go toolchain.
func ResolveModuleDir(ctx context.Context, dir, module string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	output, err := execOutput(ctx, dir, "go", "list", "-m", "-f", "{{.Dir}}", module)
	if err != nil {
		return "", fmt.Errorf("go list -m %s: %w (%s)", module, err, strings.TrimSpace(string(output)))
	}
	resolved := strings.TrimSpace(string(output))
	if resolved == "" {
		return "", fmt.Errorf("go list -m %s returned no directory", module)
	}
	return resolved, nil
}
