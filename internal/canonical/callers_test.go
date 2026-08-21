package canonical

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

const testHomeSHA = "89be739ee8a1d1ed6ebbe97dd1556a253477d242"

func testHome() HomePin {
	return HomePin{Repository: "t33n-software/repository-governance", SHA: testHomeSHA}
}

func callerContents(pin string) string {
	return "name: CI\n\njobs:\n  quality:\n    uses: t33n-software/repository-governance/.github/workflows/reusable-ci-go.yml@" + pin + "\n"
}

// callerFixture binds the seams for the caller-proof tests.
type callerFixture struct {
	tenantContents map[string][]byte
	tenantErr      error
	homeContents   map[string][]byte
	homeErr        error
	recordContents []byte
	recordErr      error
}

func (fixture callerFixture) verifier() Verifier {
	return Verifier{
		ReadTenant: func(path string) ([]byte, error) {
			if fixture.tenantErr != nil {
				return nil, fixture.tenantErr
			}
			contents, found := fixture.tenantContents[path]
			if !found {
				return nil, fmt.Errorf("no such tenant file: %s", path)
			}
			return contents, nil
		},
		ReadHome: func(path string) ([]byte, error) {
			if fixture.homeErr != nil {
				return nil, fixture.homeErr
			}
			if path == callerHashesPath {
				return fixture.recordContents, fixture.recordErr
			}
			contents, found := fixture.homeContents[path]
			if !found {
				return nil, fmt.Errorf("no such home file: %s", path)
			}
			return contents, nil
		},
	}
}

func callerRecordJSON(master, hash string) string {
	return `{
  "schemaVersion": 1,
  "home": { "repository": "t33n-software/repository-governance", "sha": "` + testHomeSHA + `" },
  "callers": [
    { "master": "` + master + `", "tenantFile": ".github/workflows/ci.yml", "class": "linux-only", "sha256": "` + hash + `" }
  ]
}`
}

func passingCallerFixture() callerFixture {
	tenant := callerContents(testHomeSHA)
	master := tenant
	hash := Sum256Hex([]byte(master))
	return callerFixture{
		tenantContents: map[string][]byte{".github/workflows/ci.yml": []byte(tenant)},
		homeContents:   map[string][]byte{"hosting-platforms/github/workflows/callers/go/ci.yml": []byte(master)},
		recordContents: []byte(callerRecordJSON("hosting-platforms/github/workflows/callers/go/ci.yml", hash)),
	}
}

func passingCallerBinding() Bindings {
	master := callerContents(testHomeSHA)
	return Bindings{
		Home: testHome(),
		Callers: []CallerBinding{{
			File:   ".github/workflows/ci.yml",
			Master: "hosting-platforms/github/workflows/callers/go/ci.yml",
			SHA256: Sum256Hex([]byte(master)),
		}},
	}
}

func TestVerifyCallersPass(t *testing.T) {
	fixture := passingCallerFixture()
	findings := fixture.verifier().verifyCallers(passingCallerBinding())
	if len(findings) != 0 {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyCallersRecordReadError(t *testing.T) {
	fixture := passingCallerFixture()
	fixture.recordErr = errors.New("boom")
	findings := fixture.verifier().verifyCallers(passingCallerBinding())
	if len(findings) != 1 || findings[0].Check != "callers" {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyCallersTenantReadError(t *testing.T) {
	fixture := passingCallerFixture()
	fixture.tenantErr = errors.New("boom")
	findings := fixture.verifier().verifyCallers(passingCallerBinding())
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, "boom") {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyCallersTenantHashMismatch(t *testing.T) {
	fixture := passingCallerFixture()
	fixture.tenantContents[".github/workflows/ci.yml"] = []byte("drifted")
	findings := fixture.verifier().verifyCallers(passingCallerBinding())
	assertFindingContains(t, findings, "the tenant caller hash")
}

func TestVerifyCallersMasterReadError(t *testing.T) {
	fixture := passingCallerFixture()
	fixture.homeContents = map[string][]byte{}
	findings := fixture.verifier().verifyCallers(passingCallerBinding())
	assertFindingContains(t, findings, "no such home file")
}

func TestVerifyCallersMasterHashMismatch(t *testing.T) {
	fixture := passingCallerFixture()
	fixture.homeContents["hosting-platforms/github/workflows/callers/go/ci.yml"] = []byte("drifted")
	findings := fixture.verifier().verifyCallers(passingCallerBinding())
	assertFindingContains(t, findings, "the canonical master hash")
}

func TestVerifyCallersPublishedMissing(t *testing.T) {
	fixture := passingCallerFixture()
	fixture.recordContents = []byte(callerRecordJSON("hosting-platforms/github/workflows/callers/go/other.yml", Sum256Hex([]byte("x"))))
	findings := fixture.verifier().verifyCallers(passingCallerBinding())
	assertFindingContains(t, findings, "carries no entry")
}

func TestVerifyCallersPublishedMismatch(t *testing.T) {
	fixture := passingCallerFixture()
	fixture.recordContents = []byte(callerRecordJSON("hosting-platforms/github/workflows/callers/go/ci.yml", strings.Repeat("0", 64)))
	findings := fixture.verifier().verifyCallers(passingCallerBinding())
	assertFindingContains(t, findings, "the published caller hash")
}

func TestVerifyCallersPinMismatch(t *testing.T) {
	fixture := passingCallerFixture()
	fixture.tenantContents[".github/workflows/ci.yml"] = []byte(callerContents(strings.Repeat("0", 40)))
	// Keep the declared hash consistent with the drifted tenant file so only
	// the pin proof fails.
	bindings := passingCallerBinding()
	bindings.Callers[0].SHA256 = Sum256Hex(fixture.tenantContents[".github/workflows/ci.yml"])
	fixture.homeContents["hosting-platforms/github/workflows/callers/go/ci.yml"] = fixture.tenantContents[".github/workflows/ci.yml"]
	fixture.recordContents = []byte(callerRecordJSON("hosting-platforms/github/workflows/callers/go/ci.yml", bindings.Callers[0].SHA256))
	findings := fixture.verifier().verifyCallers(bindings)
	assertFindingContains(t, findings, "diverges from the bound home SHA")
}

func TestVerifyCallerPin(t *testing.T) {
	home := testHome()

	t.Run("pass", func(t *testing.T) {
		if err := verifyCallerPin(callerContents(testHomeSHA), home); err != nil {
			t.Fatalf("verifyCallerPin: %v", err)
		}
	})

	t.Run("no uses line", func(t *testing.T) {
		if err := verifyCallerPin("name: CI\n", home); err == nil || !strings.Contains(err.Error(), "no uses reference") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("missing pin", func(t *testing.T) {
		err := verifyCallerPin("jobs:\n  quality:\n    uses: t33n-software/repository-governance/.github/workflows/reusable-ci-go.yml\n", home)
		if err == nil || !strings.Contains(err.Error(), "carries no pin") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("wrong home", func(t *testing.T) {
		err := verifyCallerPin("jobs:\n  quality:\n    uses: example.com/other/.github/workflows/reusable-ci-go.yml@"+testHomeSHA+"\n", home)
		if err == nil || !strings.Contains(err.Error(), "not the bound home") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("wrong sha", func(t *testing.T) {
		err := verifyCallerPin(callerContents(strings.Repeat("0", 40)), home)
		if err == nil || !strings.Contains(err.Error(), "diverges from the bound home SHA") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestPublishedCallerHashesErrors(t *testing.T) {
	t.Run("read error", func(t *testing.T) {
		verifier := Verifier{ReadHome: func(path string) ([]byte, error) { return nil, errors.New("boom") }}
		if _, err := verifier.publishedCallerHashes(); err == nil {
			t.Fatal("expected the read error")
		}
	})

	t.Run("decode error", func(t *testing.T) {
		verifier := Verifier{ReadHome: func(path string) ([]byte, error) { return []byte("not json"), nil }}
		if _, err := verifier.publishedCallerHashes(); err == nil {
			t.Fatal("expected the decode error")
		}
	})
}

func TestDecodeCallerHashes(t *testing.T) {
	valid := callerRecordJSON("hosting-platforms/github/workflows/callers/go/ci.yml", strings.Repeat("a", 64))
	if _, err := decodeCallerHashes([]byte(valid)); err != nil {
		t.Fatalf("decodeCallerHashes: %v", err)
	}

	tests := []struct {
		name string
		doc  string
	}{
		{name: "empty", doc: ``},
		{name: "not json", doc: `not json`},
		{name: "trailing document", doc: valid + ` {}`},
		{name: "wrong schema version", doc: `{"schemaVersion":2,"callers":[]}`},
		{name: "no callers", doc: `{"schemaVersion":1,"home":{"repository":"a/b","sha":"` + testHomeSHA + `"},"callers":[]}`},
		{name: "missing fields", doc: `{"schemaVersion":1,"callers":[{"master":"","tenantFile":"f","sha256":"` + strings.Repeat("a", 64) + `"}]}`},
		{name: "bad hash", doc: `{"schemaVersion":1,"callers":[{"master":"m","tenantFile":"f","sha256":"zz"}]}`},
		{name: "duplicate master", doc: `{"schemaVersion":1,"callers":[{"master":"m","tenantFile":"f","sha256":"` + strings.Repeat("a", 64) + `"},{"master":"m","tenantFile":"g","sha256":"` + strings.Repeat("b", 64) + `"}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeCallerHashes([]byte(test.doc)); err == nil {
				t.Fatal("expected a rejection")
			}
		})
	}
}

// assertFindingContains fails the test when no finding carries the fragment.
func assertFindingContains(t *testing.T, findings []Finding, fragment string) {
	t.Helper()
	for _, finding := range findings {
		if strings.Contains(finding.Detail, fragment) {
			return
		}
	}
	t.Fatalf("no finding contains %q: %v", fragment, findings)
}
