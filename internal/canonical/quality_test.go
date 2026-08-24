package canonical

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestDecodeQualityConfigVersion(t *testing.T) {
	version, err := DecodeQualityConfigVersion([]byte(`{"schemaVersion":4,"toolchain":{"language":"go","version":"1.26.6"},"gates":[{"name":"a","command":"go"}]}`))
	if err != nil {
		t.Fatalf("DecodeQualityConfigVersion: %v", err)
	}
	if version != 4 {
		t.Fatalf("version = %d", version)
	}
}

func TestDecodeQualityConfigVersionFullShape(t *testing.T) {
	full := `{
  "schemaVersion": 4,
  "toolchain": { "language": "go", "version": "1.26.6" },
  "extends": ["opentofu@1"],
  "defaults": { "includeFamilies": ["feature"], "excludeFamilies": [] },
  "gates": [
    {
      "name": "full-local-build",
      "command": "go",
      "args": ["tool", "-modfile", "tools/go.mod", "quality-gate"],
      "timeout": "15m",
      "workingDirectory": ".",
      "includeFamilies": ["feature"],
      "excludeFamilies": ["hotfix"]
    }
  ],
  "project": {
    "binaries": [{ "package": "./cmd/tool", "smoke": ["--version"] }],
    "fuzz": [{ "package": "./internal/boundary", "target": "FuzzParse", "time": "50000x" }]
  }
}`
	version, err := DecodeQualityConfigVersion([]byte(full))
	if err != nil {
		t.Fatalf("DecodeQualityConfigVersion full: %v", err)
	}
	if version != 4 {
		t.Fatalf("version = %d", version)
	}
}

func TestDecodeQualityConfigVersionRejections(t *testing.T) {
	tests := []struct {
		name string
		doc  string
	}{
		{name: "empty", doc: ``},
		{name: "not json", doc: `not json`},
		{name: "unknown field", doc: `{"schemaVersion":4,"bogus":true}`},
		{name: "trailing document", doc: `{"schemaVersion":4} {}`},
		{name: "type mismatch", doc: `{"schemaVersion":"4"}`},
		{name: "v3 wire form", doc: `{"schemaVersion":4,"toolchain":{"goVersion":"1.26.6"},"gates":[{"name":"a","command":"go"}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodeQualityConfigVersion([]byte(test.doc)); err == nil {
				t.Fatal("expected a rejection")
			}
		})
	}
}

func TestVerifyQuality(t *testing.T) {
	bindings := Bindings{Quality: QualityBinding{Config: "git-governance.quality.json", SchemaVersion: 4}}

	t.Run("pass", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) {
				return []byte(`{"schemaVersion":4,"toolchain":{"language":"go","version":"1.26.6"},"gates":[{"name":"a","command":"go"}]}`), nil
			},
		}
		if findings := verifier.verifyQuality(bindings); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("read error", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) { return nil, fs.ErrNotExist },
		}
		findings := verifier.verifyQuality(bindings)
		if len(findings) != 1 || !strings.Contains(findings[0].Detail, "read") {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("decode error", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) { return []byte(`not json`), nil },
		}
		findings := verifier.verifyQuality(bindings)
		if len(findings) != 1 {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("version mismatch", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) { return []byte(`{"schemaVersion":2}`), nil },
		}
		findings := verifier.verifyQuality(bindings)
		if len(findings) != 1 || !strings.Contains(findings[0].Detail, "schemaVersion 2") {
			t.Fatalf("findings = %v", findings)
		}
	})
}

func TestVerifyQualityReadErrorPropagates(t *testing.T) {
	boom := errors.New("boom")
	verifier := Verifier{
		ReadTenant: func(path string) ([]byte, error) { return nil, boom },
	}
	findings := verifier.verifyQuality(Bindings{Quality: QualityBinding{Config: "config.json", SchemaVersion: 4}})
	if len(findings) != 1 || !strings.Contains(findings[0].Detail, "boom") {
		t.Fatalf("findings = %v", findings)
	}
}
