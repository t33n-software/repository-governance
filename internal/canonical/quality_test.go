package canonical

import (
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

func TestDecodeQualityConfigExtends(t *testing.T) {
	extends, err := DecodeQualityConfigExtends([]byte(`{"schemaVersion":4,"toolchain":{"language":"go","version":"1.26.6"},"extends":["opentofu@1"],"gates":[{"name":"a","command":"go"}]}`))
	if err != nil {
		t.Fatalf("DecodeQualityConfigExtends: %v", err)
	}
	if len(extends) != 1 || extends[0] != "opentofu@1" {
		t.Fatalf("extends = %v", extends)
	}
}

func TestDecodeQualityConfigExtendsWithoutDeclaration(t *testing.T) {
	extends, err := DecodeQualityConfigExtends([]byte(`{"schemaVersion":4,"gates":[{"name":"a","command":"go"}]}`))
	if err != nil {
		t.Fatalf("DecodeQualityConfigExtends: %v", err)
	}
	if len(extends) != 0 {
		t.Fatalf("extends = %v", extends)
	}
}

func TestDecodeQualityConfigExtendsRejection(t *testing.T) {
	if _, err := DecodeQualityConfigExtends([]byte(`not json`)); err == nil {
		t.Fatal("expected a rejection")
	}
}

func TestReadQualitySeam(t *testing.T) {
	t.Run("pass", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) {
				return []byte(`{"schemaVersion":4,"toolchain":{"language":"go","version":"1.26.6"},"gates":[{"name":"a","command":"go"}]}`), nil
			},
		}
		seam, findings := verifier.readQualitySeam(Bindings{Quality: QualityBinding{Config: "git-governance.quality.json", SchemaVersion: 4}})
		if len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
		if seam.Toolchain.Language != "go" {
			t.Fatalf("seam = %+v", seam)
		}
	})

	t.Run("read error", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) { return nil, fs.ErrNotExist },
		}
		_, findings := verifier.readQualitySeam(Bindings{Quality: QualityBinding{Config: "git-governance.quality.json", SchemaVersion: 4}})
		if len(findings) != 1 || !strings.Contains(findings[0].Detail, "read") {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("decode error", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) { return []byte(`not json`), nil },
		}
		_, findings := verifier.readQualitySeam(Bindings{Quality: QualityBinding{Config: "git-governance.quality.json", SchemaVersion: 4}})
		if len(findings) != 1 {
			t.Fatalf("findings = %v", findings)
		}
	})
}

func TestVerifyQuality(t *testing.T) {
	t.Run("version match", func(t *testing.T) {
		verifier := Verifier{}
		seam := qualityConfigDocument{SchemaVersion: 4}
		if findings := verifier.verifyQuality(seam, Bindings{Quality: QualityBinding{Config: "git-governance.quality.json", SchemaVersion: 4}}); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("version mismatch", func(t *testing.T) {
		verifier := Verifier{}
		seam := qualityConfigDocument{SchemaVersion: 2}
		findings := verifier.verifyQuality(seam, Bindings{Quality: QualityBinding{Config: "git-governance.quality.json", SchemaVersion: 4}})
		if len(findings) != 1 || !strings.Contains(findings[0].Detail, "schemaVersion 2") {
			t.Fatalf("findings = %v", findings)
		}
	})
}
