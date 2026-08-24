package canonical

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// errTestMissing is the shared test error for absent files.
var errTestMissing = errors.New("no such file")

// testNilContext returns a nil context through a function boundary so the
// normalization branch stays whitebox-testable.
func testNilContext() context.Context {
	return nil
}

func TestVerifyPass(t *testing.T) {
	fixture := passingFilesFixture()
	callerFixture := passingCallerFixture()
	for path, contents := range callerFixture.tenantContents {
		fixture.tenantContents[path] = contents
	}
	for path, contents := range callerFixture.homeContents {
		fixture.homeContents[path] = contents
	}
	fixture.homeContents[callerHashesPath] = callerFixture.recordContents
	fixture.tenantContents["git-governance.quality.json"] = []byte(`{"schemaVersion":4,"toolchain":{"language":"go","version":"1.26.6"},"gates":[{"name":"a","command":"go"}]}`)
	fixture.tenantContents["go.mod"] = []byte("module example.com/tenant\n\ngo 1.26\n\ntoolchain go1.26.6\n")

	verifier := fixture.verifier()
	verifier.ReadHome = func(path string) ([]byte, error) {
		if path == callerHashesPath {
			return callerFixture.recordContents, nil
		}
		if path == "go.mod" {
			return []byte("module github.com/t33n-software/repository-governance\n"), nil
		}
		contents, found := fixture.homeContents[path]
		if !found {
			return nil, errTestMissing
		}
		return contents, nil
	}
	verifier.ResolveModule = func(ctx context.Context, dir, module string) (string, error) {
		return "gqa", nil
	}
	verifier.ReadModule = func(dir, path string) ([]byte, error) {
		return []byte(catalogJSON()), nil
	}
	verifier.TenantRoot = "tenant"

	bindings := passingCallerBinding()
	bindings.Files = canonicalFileBindings()
	bindings.Codeowners = CodeownersBinding{Path: ".github/CODEOWNERS", DefaultOwner: "@CyberT33N"}
	bindings.Quality = QualityBinding{Config: "git-governance.quality.json", SchemaVersion: 4}
	bindings.Tools = ToolsBinding{Module: "tools/go.mod", CatalogVersion: 1}
	fixture.tenantContents["tools/go.mod"] = []byte("module example.com/tenant/tools\n")

	// The tools module without directives passes vacuously; the license family
	// is not bound in this fixture.
	if findings := verifier.Verify(context.Background(), bindings); len(findings) != 0 {
		t.Fatalf("findings = %v", findings)
	}

	// A nil context is normalized.
	if findings := verifier.Verify(testNilContext(), bindings); len(findings) != 0 {
		t.Fatalf("findings with nil context = %v", findings)
	}
}

func TestVerifyCollectsFailures(t *testing.T) {
	verifier := Verifier{
		ReadTenant: func(path string) ([]byte, error) { return nil, errTestMissing },
		ReadHome:   func(path string) ([]byte, error) { return nil, errTestMissing },
	}
	bindings := Bindings{
		Callers:    []CallerBinding{{File: ".github/workflows/ci.yml", Master: "m", SHA256: strings.Repeat("a", 64)}},
		Files:      canonicalFileBindings(),
		Codeowners: CodeownersBinding{Path: ".github/CODEOWNERS", DefaultOwner: "@x"},
		Quality:    QualityBinding{Config: "git-governance.quality.json", SchemaVersion: 4},
		Tools:      ToolsBinding{Module: "tools/go.mod", CatalogVersion: 1},
		Class:      Class{LicenseHub: true},
	}
	findings := verifier.Verify(context.Background(), bindings)
	if len(findings) == 0 {
		t.Fatal("expected findings")
	}
}

func TestReport(t *testing.T) {
	var stdout, stderr strings.Builder
	verifier := Verifier{Stdout: &stdout, Stderr: &stderr}
	if !verifier.Report(nil) {
		t.Fatal("an empty finding set must pass")
	}
	if !strings.Contains(stdout.String(), "PASS") {
		t.Fatalf("stdout = %q", stdout.String())
	}

	if verifier.Report([]Finding{{Check: "c", Detail: "d"}}) {
		t.Fatal("a finding set must fail")
	}
	if !strings.Contains(stderr.String(), "FAIL") || !strings.Contains(stderr.String(), "c: d") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestFindingHelpers(t *testing.T) {
	read := readErrorFinding("check", "path", errTestMissing)
	if read.Check != "check" || !strings.Contains(read.Detail, "path") {
		t.Fatalf("finding = %+v", read)
	}
	mismatch := mismatchFinding("check", "detail")
	if mismatch.Check != "check" || mismatch.Detail != "detail" {
		t.Fatalf("finding = %+v", mismatch)
	}
}
