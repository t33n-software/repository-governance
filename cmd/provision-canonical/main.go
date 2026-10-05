// Command provision-canonical is the provisioning CLI of the
// repository-governance home: the write exposure of the canonical render
// core that cmd/verify-canonical proves with. It reads the tenant's
// repo-bindings.json, resolves the pinned home tree, renders every bound
// tenant surface — the byte-identical callers and canonical files, the
// composed gitignore governed region with the preserved project block, the
// materialized CODEOWNERS, the conventions README where bound, and, where
// the manifest binds the toolchain section, the pinned territory config
// artifacts and the composed pnpm workspace document — proves every copied
// master, territory artifact, and rendered region against the manifest's
// recorded hashes fail-closed, and writes the proven materializations. The
// selection is reviewable, versioned manifest data; the CLI carries no
// selection flags.
package main

import (
	"bufio"
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
	exitProcess               = os.Exit
	commandArgs               = os.Args
	readFile                  = os.ReadFile
	resolveHome               = canonical.ResolveModuleDir
	newProvisioner            = canonical.NewProvisioner
	planTenant                = planTenantMaterials
	applyTenant               = applyTenantMaterials
	stdinIsTerminal           = detectTerminal
	stdinReader     io.Reader = os.Stdin
)

func main() {
	exitProcess(run(context.Background(), commandArgs[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	root := "."
	home := ""
	territory := ""
	dryRun := false
	confirm := false
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--version":
			fmt.Fprintf(stdout, "provision-canonical %s\n", version)
			return 0
		case arg == "--dry-run":
			dryRun = true
		case arg == "--yes":
			confirm = true
		case arg == "--repo" || arg == "--home" || arg == "--territory-home":
			value, ok := flagValue(args, &index)
			if !ok {
				fmt.Fprintf(stderr, "usage: provision-canonical [--repo <path>] [--home <path>] [--territory-home <path>] [--dry-run] [--yes] [--version]\n")
				return 2
			}
			switch arg {
			case "--repo":
				root = value
			case "--home":
				home = value
			default:
				territory = value
			}
		case strings.HasPrefix(arg, "--repo="):
			root = strings.TrimPrefix(arg, "--repo=")
		case strings.HasPrefix(arg, "--home="):
			home = strings.TrimPrefix(arg, "--home=")
		case strings.HasPrefix(arg, "--territory-home="):
			territory = strings.TrimPrefix(arg, "--territory-home=")
		default:
			fmt.Fprintf(stderr, "usage: provision-canonical [--repo <path>] [--home <path>] [--territory-home <path>] [--dry-run] [--yes] [--version]\n")
			return 2
		}
	}

	bindings, code := decodeTenantBindings(root, stderr)
	if code != 0 {
		return code
	}

	homeRoot, err := resolveHomeRoot(ctx, root, home, bindings)
	if err != nil {
		fmt.Fprintf(stderr, "provision-canonical: %v\n", err)
		return 1
	}
	territoryRoot, err := resolveTerritoryRoot(territory, bindings)
	if err != nil {
		fmt.Fprintf(stderr, "provision-canonical: %v\n", err)
		return 1
	}

	provisioner := newProvisioner(root, homeRoot, territoryRoot)

	if dryRun {
		materials, err := planTenant(provisioner, bindings)
		if err != nil {
			fmt.Fprintf(stderr, "provision-canonical: %v\n", err)
			return 1
		}
		writePlan(stdout, materials)
		return 0
	}

	if !confirm {
		if !stdinIsTerminal() {
			fmt.Fprintln(stderr, "provision-canonical: provisioning mutates the working tree and requires an explicit confirmation in a non-interactive context")
			fmt.Fprintln(stderr, "pass --yes to confirm the mutation, or --dry-run to preview the plan")
			return 2
		}
		materials, err := planTenant(provisioner, bindings)
		if err != nil {
			fmt.Fprintf(stderr, "provision-canonical: %v\n", err)
			return 1
		}
		writePlan(stdout, materials)
		fmt.Fprint(stdout, "Apply the provisioning? [y/N] ")
		answer, _ := bufio.NewReader(stdinReader).ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(stderr, "provision-canonical: the confirmation was declined; no files were written")
			return 2
		}
	}

	materials, err := applyTenant(provisioner, bindings)
	if err != nil {
		fmt.Fprintf(stderr, "provision-canonical: %v\n", err)
		return 1
	}
	for _, material := range materials {
		fmt.Fprintf(stdout, "wrote %s\n", material.Path)
	}
	fmt.Fprintf(stdout, "Canonical provisioning: PASS (%d files written)\n", len(materials))
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

// writePlan writes the dry-run plan of the provisioning.
func writePlan(stdout io.Writer, materials []canonical.Materialization) {
	fmt.Fprintf(stdout, "Canonical provisioning plan: no files written (%d)\n", len(materials))
	for _, material := range materials {
		fmt.Fprintf(stdout, "would write %s (from %s)\n", material.Path, material.Source)
	}
}

// decodeTenantBindings reads and strictly decodes the tenant's binding
// manifest.
func decodeTenantBindings(root string, stderr io.Writer) (canonical.Bindings, int) {
	contents, err := readFile(filepath.Join(root, bindingsFileName))
	if err != nil {
		fmt.Fprintf(stderr, "provision-canonical: read %s: %v\n", bindingsFileName, err)
		return canonical.Bindings{}, 1
	}
	bindings, err := canonical.DecodeBindings(contents)
	if err != nil {
		fmt.Fprintf(stderr, "provision-canonical: %v\n", err)
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

// resolveTerritoryRoot binds the territory tree: the explicit
// --territory-home flag wins; a tenant that binds a toolchain section
// without the flag is a fail-closed resolution error, because the config
// topics would otherwise claim writes no seam carries. The semantics mirror
// the verifier's resolveTerritoryRoot exactly.
func resolveTerritoryRoot(territory string, bindings canonical.Bindings) (string, error) {
	if territory != "" {
		return territory, nil
	}
	if bindings.Toolchain != nil {
		return "", fmt.Errorf("the binding manifest binds a toolchain section; pass --territory-home <path>")
	}
	return "", nil
}

// planTenantMaterials is the default planning seam.
func planTenantMaterials(provisioner canonical.Provisioner, bindings canonical.Bindings) ([]canonical.Materialization, error) {
	return provisioner.Plan(bindings)
}

// applyTenantMaterials is the default apply seam.
func applyTenantMaterials(provisioner canonical.Provisioner, bindings canonical.Bindings) ([]canonical.Materialization, error) {
	return provisioner.Apply(bindings)
}

// detectTerminal reports whether the standard input is an interactive
// terminal.
func detectTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
