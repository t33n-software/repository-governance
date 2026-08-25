package canonical

import (
	"context"
	"fmt"
	"io"
)

// Finding is one failed conformance proof.
type Finding struct {
	Check  string
	Detail string
}

// Verifier proves a tenant's canonical bindings fail-closed. Every seam is
// injected so each proof stays whitebox-testable without a real tree; the
// production bindings are constructed by NewVerifier.
type Verifier struct {
	// TenantRoot is the repository root under verification.
	TenantRoot string
	// ReadTenant reads a tenant file by its repository-relative slash path.
	ReadTenant func(path string) ([]byte, error)
	// ReadHome reads a home file by its home-relative slash path.
	ReadHome func(path string) ([]byte, error)
	// ReadModule reads a file inside a resolved module directory.
	ReadModule func(dir, path string) ([]byte, error)
	// ListTenant lists the entry names of a tenant directory by its
	// repository-relative slash path.
	ListTenant func(path string) ([]string, error)
	// ListModule lists the entry names of a directory inside a resolved
	// module directory.
	ListModule func(dir, path string) ([]string, error)
	// ResolveModule resolves a module's cache directory within the tenant's
	// tooling module context.
	ResolveModule func(ctx context.Context, dir, module string) (string, error)
	// Stdout receives the per-check report; Stderr receives the findings.
	Stdout io.Writer
	Stderr io.Writer
}

// Verify runs every bound proof family and returns the collected findings. An
// empty result is the only pass; missing or diverging evidence is a finding,
// never a pass.
func (v Verifier) Verify(ctx context.Context, bindings Bindings) []Finding {
	if ctx == nil {
		ctx = context.Background()
	}
	findings := make([]Finding, 0)
	findings = append(findings, v.verifyCallers(bindings)...)
	findings = append(findings, v.verifyFiles(bindings)...)
	findings = append(findings, v.verifyCodeowners(bindings)...)
	findings = append(findings, v.verifyQuality(bindings)...)
	findings = append(findings, v.verifyExtends(ctx, bindings)...)
	findings = append(findings, v.verifyToolchain()...)
	findings = append(findings, v.verifyTools(ctx, bindings)...)
	findings = append(findings, v.verifyLicense(bindings)...)
	return findings
}

// Report writes the per-check outcome and returns whether the verification
// passed.
func (v Verifier) Report(findings []Finding) bool {
	if len(findings) == 0 {
		fmt.Fprintln(v.Stdout, "Canonical conformance: PASS")
		return true
	}
	for _, finding := range findings {
		fmt.Fprintf(v.Stderr, "%s: %s\n", finding.Check, finding.Detail)
	}
	fmt.Fprintf(v.Stderr, "Canonical conformance: FAIL (%d findings)\n", len(findings))
	return false
}

// readErrorFinding converts a read failure into a fail-closed finding.
func readErrorFinding(check, path string, err error) Finding {
	return Finding{Check: check, Detail: fmt.Sprintf("read %s: %v", path, err)}
}

// mismatchFinding reports a content divergence.
func mismatchFinding(check, detail string) Finding {
	return Finding{Check: check, Detail: detail}
}
