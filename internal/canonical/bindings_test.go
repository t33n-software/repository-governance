package canonical

import (
	"strings"
	"testing"
)

// validBindingsJSON is the reference manifest every mutation starts from.
func validBindingsJSON() string {
	return `{
  "schemaVersion": 1,
  "home": {
    "repository": "t33n-software/repository-governance",
    "version": "v1.0.0",
    "sha": "89be739ee8a1d1ed6ebbe97dd1556a253477d242"
  },
  "class": {
    "qualityGates": "linux-only",
    "codeScanning": true,
    "licenseHub": false
  },
  "callers": [
    {
      "file": ".github/workflows/ci.yml",
      "master": "hosting-platforms/github/workflows/callers/go/ci.yml",
      "sha256": "f29a65bd73fe575b159123a9d4bebed86ab4eebe3c5dc5dac31c96e7fb7c4c4a"
    }
  ],
  "files": {
    "lefthook": { "path": "lefthook.yml", "sha256": "` + strings.Repeat("a", 64) + `" },
    "gitattributes": { "path": ".gitattributes", "sha256": "` + strings.Repeat("b", 64) + `" },
    "gitignore": { "path": ".gitignore", "sha256": "` + strings.Repeat("c", 64) + `" },
    "dependabot": { "path": ".github/dependabot.yml", "sha256": "` + strings.Repeat("d", 64) + `" }
  },
  "codeowners": { "path": ".github/CODEOWNERS", "defaultOwner": "@CyberT33N" },
  "quality": { "config": "git-governance.quality.json", "schemaVersion": 4 },
  "tools": { "module": "tools/go.mod", "catalogVersion": 1 }
}`
}

func TestDecodeBindingsAcceptsTheReferenceManifest(t *testing.T) {
	bindings, err := DecodeBindings([]byte(validBindingsJSON()))
	if err != nil {
		t.Fatalf("DecodeBindings: %v", err)
	}
	if bindings.SchemaVersion != 1 {
		t.Fatalf("SchemaVersion = %d", bindings.SchemaVersion)
	}
	if bindings.Home.Repository != "t33n-software/repository-governance" {
		t.Fatalf("Home.Repository = %q", bindings.Home.Repository)
	}
	if bindings.Home.Version != "v1.0.0" {
		t.Fatalf("Home.Version = %q", bindings.Home.Version)
	}
	if bindings.Class.QualityGates != "linux-only" || !bindings.Class.CodeScanning || bindings.Class.LicenseHub {
		t.Fatalf("Class = %+v", bindings.Class)
	}
	if len(bindings.Callers) != 1 {
		t.Fatalf("Callers = %+v", bindings.Callers)
	}
	if bindings.Files.Lefthook.Path != "lefthook.yml" {
		t.Fatalf("Files.Lefthook = %+v", bindings.Files.Lefthook)
	}
	if bindings.Codeowners.DefaultOwner != "@CyberT33N" {
		t.Fatalf("Codeowners = %+v", bindings.Codeowners)
	}
	if bindings.Conventions != nil {
		t.Fatalf("Conventions = %+v", bindings.Conventions)
	}
	if bindings.Quality.SchemaVersion != 4 {
		t.Fatalf("Quality = %+v", bindings.Quality)
	}
	if bindings.Tools.Module != "tools/go.mod" || bindings.Tools.CatalogVersion != 1 {
		t.Fatalf("Tools = %+v", bindings.Tools)
	}
}

func TestDecodeBindingsAcceptsAnOptionalVersionlessHome(t *testing.T) {
	manifest := strings.Replace(validBindingsJSON(), `"version": "v1.0.0",`, `"version": "",`, 1)
	if _, err := DecodeBindings([]byte(manifest)); err != nil {
		t.Fatalf("DecodeBindings without a version: %v", err)
	}
}

// validBindingsWithConventionsJSON carries the optional conventions section.
func validBindingsWithConventionsJSON() string {
	return strings.Replace(validBindingsJSON(), `  "codeowners": { "path": ".github/CODEOWNERS", "defaultOwner": "@CyberT33N" },`, `  "codeowners": { "path": ".github/CODEOWNERS", "defaultOwner": "@CyberT33N" },
  "conventions": { "path": "docs/conventions/hosting-platforms/github/rule-sets/README.md", "organization": "t33n-software", "repository": "supply-chain-governance", "rationale": "Portable schemas only." },`, 1)
}

func TestDecodeBindingsAcceptsConventions(t *testing.T) {
	bindings, err := DecodeBindings([]byte(validBindingsWithConventionsJSON()))
	if err != nil {
		t.Fatalf("DecodeBindings: %v", err)
	}
	if bindings.Conventions == nil {
		t.Fatal("Conventions must be bound")
	}
	if bindings.Conventions.Path != "docs/conventions/hosting-platforms/github/rule-sets/README.md" {
		t.Fatalf("Conventions.Path = %q", bindings.Conventions.Path)
	}
	if bindings.Conventions.Organization != "t33n-software" {
		t.Fatalf("Conventions.Organization = %q", bindings.Conventions.Organization)
	}
	if bindings.Conventions.Repository != "supply-chain-governance" {
		t.Fatalf("Conventions.Repository = %q", bindings.Conventions.Repository)
	}
	if bindings.Conventions.Rationale != "Portable schemas only." {
		t.Fatalf("Conventions.Rationale = %q", bindings.Conventions.Rationale)
	}
}

func TestDecodeBindingsConventionsRejections(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(string) string
		message string
	}{
		{
			name: "empty organization",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"organization": "t33n-software"`, `"organization": ""`, 1)
			},
			message: "conventions.organization",
		},
		{
			name: "empty repository",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"repository": "supply-chain-governance"`, `"repository": ""`, 1)
			},
			message: "conventions.repository",
		},
		{
			name: "empty rationale",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"rationale": "Portable schemas only."`, `"rationale": ""`, 1)
			},
			message: "conventions.rationale",
		},
		{
			name: "bad path",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"path": "docs/conventions/hosting-platforms/github/rule-sets/README.md"`, `"path": "../README.md"`, 1)
			},
			message: "conventions.path",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeBindings([]byte(test.mutate(validBindingsWithConventionsJSON())))
			if err == nil {
				t.Fatalf("expected rejection containing %q", test.message)
			}
			if !strings.Contains(err.Error(), test.message) {
				t.Fatalf("error %q does not contain %q", err.Error(), test.message)
			}
		})
	}
}

func TestDecodeBindingsRejections(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(string) string
		message string
	}{
		{
			name:    "wrong schema version",
			mutate:  func(doc string) string { return strings.Replace(doc, `"schemaVersion": 1`, `"schemaVersion": 2`, 1) },
			message: "schemaVersion must equal 1",
		},
		{
			name: "unknown field",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"schemaVersion": 1,`, `"schemaVersion": 1, "bogus": true,`, 1)
			},
			message: "known fields",
		},
		{
			name: "bad home repository",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"t33n-software/repository-governance"`, `"T33N"`, 1)
			},
			message: "home.repository",
		},
		{
			name: "bad home sha",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"89be739ee8a1d1ed6ebbe97dd1556a253477d242"`, `"89be739"`, 1)
			},
			message: "home.sha",
		},
		{
			name:    "bad home version",
			mutate:  func(doc string) string { return strings.Replace(doc, `"version": "v1.0.0"`, `"version": "1.0.0"`, 1) },
			message: "home.version",
		},
		{
			name: "bad class",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"qualityGates": "linux-only"`, `"qualityGates": "windows-only"`, 1)
			},
			message: "class.qualityGates",
		},
		{
			name: "bad caller file",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"file": ".github/workflows/ci.yml"`, `"file": "ci.yml"`, 1)
			},
			message: "callers file",
		},
		{
			name: "bad caller master",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"master": "hosting-platforms/github/workflows/callers/go/ci.yml"`, `"master": "ci.yml"`, 1)
			},
			message: "callers master",
		},
		{
			name: "bad caller hash",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"f29a65bd73fe575b159123a9d4bebed86ab4eebe3c5dc5dac31c96e7fb7c4c4a"`, `"f29a"`, 1)
			},
			message: "callers sha256",
		},
		{
			name: "duplicate caller file",
			mutate: func(doc string) string {
				caller := `{
      "file": ".github/workflows/ci.yml",
      "master": "hosting-platforms/github/workflows/callers/go/ci.yml",
      "sha256": "f29a65bd73fe575b159123a9d4bebed86ab4eebe3c5dc5dac31c96e7fb7c4c4a"
    }`
				return strings.Replace(doc, caller, caller+","+caller, 1)
			},
			message: "must be unique",
		},
		{
			name: "duplicate caller master",
			mutate: func(doc string) string {
				second := `{
      "file": ".github/workflows/codeql.yml",
      "master": "hosting-platforms/github/workflows/callers/go/ci.yml",
      "sha256": "f29a65bd73fe575b159123a9d4bebed86ab4eebe3c5dc5dac31c96e7fb7c4c4a"
    }`
				return strings.Replace(doc, `  ],
  "files"`, ",\n"+second+`  ],
  "files"`, 1)
			},
			message: "master must be unique",
		},
		{
			name: "bad codeowners path",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"path": ".github/CODEOWNERS"`, `"path": "../CODEOWNERS"`, 1)
			},
			message: "codeowners.path",
		},
		{
			name: "bad quality config path",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"config": "git-governance.quality.json"`, `"config": ""`, 1)
			},
			message: "quality.config",
		},
		{
			name: "bad files hash",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"sha256": "`+strings.Repeat("a", 64)+`"`, `"sha256": "aa"`, 1)
			},
			message: "files.lefthook.sha256",
		},
		{
			name: "empty codeowners owner",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"defaultOwner": "@CyberT33N"`, `"defaultOwner": ""`, 1)
			},
			message: "codeowners.defaultOwner",
		},
		{
			name: "wrong quality schema version",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"schemaVersion": 4 }`, `"schemaVersion": 3 }`, 1)
			},
			message: "quality.schemaVersion",
		},
		{
			name:    "wrong tools catalog version",
			mutate:  func(doc string) string { return strings.Replace(doc, `"catalogVersion": 1`, `"catalogVersion": 2`, 1) },
			message: "tools.catalogVersion",
		},
		{
			name:    "empty tools module path",
			mutate:  func(doc string) string { return strings.Replace(doc, `"module": "tools/go.mod"`, `"module": ""`, 1) },
			message: "tools.module",
		},
		{
			name: "parent traversal in a path",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"path": "lefthook.yml"`, `"path": "../lefthook.yml"`, 1)
			},
			message: "parent traversal",
		},
		{
			name: "absolute path",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"path": "lefthook.yml"`, `"path": "/lefthook.yml"`, 1)
			},
			message: "repository-relative",
		},
		{
			name: "windows absolute path",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"path": "lefthook.yml"`, `"path": "C:/lefthook.yml"`, 1)
			},
			message: "repository-relative",
		},
		{
			name: "backslash path",
			mutate: func(doc string) string {
				return strings.Replace(doc, `"path": "lefthook.yml"`, `"path": "docs\\\\lefthook.yml"`, 1)
			},
			message: "forward slashes",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodeBindings([]byte(test.mutate(validBindingsJSON())))
			if err == nil {
				t.Fatalf("expected rejection containing %q", test.message)
			}
			if !strings.Contains(err.Error(), test.message) {
				t.Fatalf("error %q does not contain %q", err.Error(), test.message)
			}
		})
	}
}

func TestDecodeBindingsRejectsMalformedDocuments(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{name: "empty", doc: ``},
		{name: "not json", doc: `not json`},
		{name: "trailing document", doc: validBindingsJSON() + ` {}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeBindings([]byte(test.doc)); err == nil {
				t.Fatal("expected a rejection")
			}
		})
	}
}

func TestDecodeBindingsRejectsOversizedDocuments(t *testing.T) {
	oversized := `{"schemaVersion":1,"pad":"` + strings.Repeat("x", maxBindingsBytes) + `"}`
	if _, err := DecodeBindings([]byte(oversized)); err == nil {
		t.Fatal("expected the size guard to reject the document")
	}
}

func TestDecodeBindingsRejectsEmptyAndOverflowingCallers(t *testing.T) {
	singleCallerBlock := `[
    {
      "file": ".github/workflows/ci.yml",
      "master": "hosting-platforms/github/workflows/callers/go/ci.yml",
      "sha256": "f29a65bd73fe575b159123a9d4bebed86ab4eebe3c5dc5dac31c96e7fb7c4c4a"
    }
  ]`
	empty := strings.Replace(validBindingsJSON(), singleCallerBlock, `[]`, 1)
	if _, err := DecodeBindings([]byte(empty)); err == nil {
		t.Fatal("expected the empty callers rejection")
	}

	entries := make([]string, 0, maxCallerCount+1)
	for index := 0; index <= maxCallerCount; index++ {
		entries = append(entries, `{"file":".github/workflows/ci-`+strings.Repeat("x", index+1)+`.yml","master":"hosting-platforms/github/workflows/callers/go/ci-`+strings.Repeat("x", index+1)+`.yml","sha256":"f29a65bd73fe575b159123a9d4bebed86ab4eebe3c5dc5dac31c96e7fb7c4c4a"}`)
	}
	overflow := strings.Replace(validBindingsJSON(), singleCallerBlock, `[`+strings.Join(entries, ",")+`]`, 1)
	if _, err := DecodeBindings([]byte(overflow)); err == nil {
		t.Fatal("expected the caller count guard to reject the document")
	}
}

func TestValidateManifestPathBranches(t *testing.T) {
	if err := validateManifestPath("field", ""); err == nil {
		t.Fatal("expected the empty path rejection")
	}
	if err := validateManifestPath("field", "docs/readme.md"); err != nil {
		t.Fatalf("expected a valid relative path, got %v", err)
	}
	if err := validateManifestPath("field", "a\x01b"); err == nil {
		t.Fatal("expected the control character rejection")
	}
}

func TestSplitPath(t *testing.T) {
	segments := splitPath("a/b/c")
	if len(segments) != 3 || segments[0] != "a" || segments[1] != "b" || segments[2] != "c" {
		t.Fatalf("splitPath = %v", segments)
	}
	if got := splitPath(""); len(got) != 1 || got[0] != "" {
		t.Fatalf("splitPath empty = %v", got)
	}
}
