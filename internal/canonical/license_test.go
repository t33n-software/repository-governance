package canonical

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestVerifyLicenseNotBound(t *testing.T) {
	verifier := Verifier{
		ReadTenant: func(path string) ([]byte, error) {
			t.Fatalf("no tenant read expected, got %q", path)
			return nil, nil
		},
	}
	if findings := verifier.verifyLicense(Bindings{Class: Class{LicenseHub: false}}); findings != nil {
		t.Fatalf("findings = %v", findings)
	}
}

func TestVerifyLicenseBound(t *testing.T) {
	bindings := Bindings{Class: Class{LicenseHub: true}}

	t.Run("pass", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) { return []byte(`{}`), nil },
		}
		if findings := verifier.verifyLicense(bindings); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) { return nil, fs.ErrNotExist },
		}
		findings := verifier.verifyLicense(bindings)
		if len(findings) != 2 {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) { return []byte(`not json`), nil },
		}
		findings := verifier.verifyLicense(bindings)
		if len(findings) != 2 {
			t.Fatalf("findings = %v", findings)
		}
		if !strings.Contains(findings[0].Detail, "valid JSON") {
			t.Fatalf("findings = %v", findings)
		}
	})
}

func TestDecodeJSONDocument(t *testing.T) {
	if err := decodeJSONDocument([]byte(`{"a":1}`)); err != nil {
		t.Fatalf("decodeJSONDocument: %v", err)
	}
	if err := decodeJSONDocument([]byte(`[1,2]`)); err != nil {
		t.Fatalf("decodeJSONDocument array: %v", err)
	}
	if err := decodeJSONDocument(nil); err == nil {
		t.Fatal("expected the empty rejection")
	}
	if err := decodeJSONDocument([]byte(`not json`)); err == nil {
		t.Fatal("expected the invalid rejection")
	}
	if err := decodeJSONDocument([]byte(`{} {}`)); err == nil {
		t.Fatal("expected the trailing document rejection")
	}
}

func TestVerifyLicenseReadErrorPropagates(t *testing.T) {
	boom := errors.New("boom")
	verifier := Verifier{
		ReadTenant: func(path string) ([]byte, error) { return nil, boom },
	}
	findings := verifier.verifyLicense(Bindings{Class: Class{LicenseHub: true}})
	if len(findings) != 2 || !strings.Contains(findings[0].Detail, "boom") {
		t.Fatalf("findings = %v", findings)
	}
}
