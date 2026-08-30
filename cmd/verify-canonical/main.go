// Command verify-canonical is the conformance verifier of the
// repository-governance home. It proves a tenant's canonical bindings
// fail-closed: caller hashes and pins, canonical file equality, CODEOWNERS
// materialization, config-seam conformance, capability-pack resolution
// against the registry at the tenant's pinned tool stand, tool-pin admission,
// and the license content proof through the tenant-pinned hub CLI.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/t33n-software/repository-governance/internal/canonical"
)

// bindingsFileName is the canonical tenant binding manifest name.
const bindingsFileName = "repo-bindings.json"

// version is the build-stamped tool version.
var version = "dev"

var (
	exitProcess = os.Exit
	commandArgs = os.Args
	readFile    = os.ReadFile
	verify      = verifyTenant
	resolveHome = canonical.ResolveModuleDir
	newVerifier = canonical.NewVerifier
)

func main() {
	exitProcess(run(context.Background(), commandArgs[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	root := "."
	home := ""
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--version":
			fmt.Fprintf(stdout, "verify-canonical %s\n", version)
			return 0
		case arg == "--repo" || arg == "--home":
			value, ok := flagValue(args, &index)
			if !ok {
				fmt.Fprintf(stderr, "usage: verify-canonical [--repo <path>] [--home <path>] [--version]\n")
				return 2
			}
			if arg == "--repo" {
				root = value
			} else {
				home = value
			}
		case strings.HasPrefix(arg, "--repo="):
			root = strings.TrimPrefix(arg, "--repo=")
		case strings.HasPrefix(arg, "--home="):
			home = strings.TrimPrefix(arg, "--home=")
		default:
			fmt.Fprintf(stderr, "usage: verify-canonical [--repo <path>] [--home <path>] [--version]\n")
			return 2
		}
	}

	bindings, code := decodeTenantBindings(root, stderr)
	if code != 0 {
		return code
	}

	homeRoot, err := resolveHomeRoot(ctx, root, home, bindings)
	if err != nil {
		fmt.Fprintf(stderr, "verify-canonical: %v\n", err)
		return 1
	}

	verifier := newVerifier(root, homeRoot, stdout, stderr)
	findings := verify(ctx, verifier, bindings)
	if !verifier.Report(findings) {
		return 1
	}
	return 0
}

// flagValue consumes the value of a space-separated flag form, advancing the
// argument index; a flag without a following value is a usage error.
func flagValue(args []string, index *int) (string, bool) {
	if *index+1 >= len(args) {
		return "", false
	}
	*index = *index + 1
	return args[*index], true
}

// decodeTenantBindings reads and strictly decodes the tenant's binding
// manifest.
func decodeTenantBindings(root string, stderr io.Writer) (canonical.Bindings, int) {
	contents, err := readFile(filepath.Join(root, bindingsFileName))
	if err != nil {
		fmt.Fprintf(stderr, "verify-canonical: read %s: %v\n", bindingsFileName, err)
		return canonical.Bindings{}, 1
	}
	bindings, err := canonical.DecodeBindings(contents)
	if err != nil {
		fmt.Fprintf(stderr, "verify-canonical: %v\n", err)
		return canonical.Bindings{}, 1
	}
	return bindings, 0
}

// resolveHomeRoot binds the home tree: the explicit --home flag wins; without
// it, the pinned home module is resolved through the tenant's tooling module.
func resolveHomeRoot(ctx context.Context, root, home string, bindings canonical.Bindings) (string, error) {
	if home != "" {
		return home, nil
	}
	toolsDir := filepath.Join(root, path.Dir(bindings.Tools.Module))
	return resolveHome(ctx, toolsDir, "github.com/"+bindings.Home.Repository)
}

// verifyTenant is the default verification seam.
func verifyTenant(ctx context.Context, verifier canonical.Verifier, bindings canonical.Bindings) []canonical.Finding {
	return verifier.Verify(ctx, bindings)
}
