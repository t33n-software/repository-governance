package canonical

import (
	"errors"
	"strings"
	"testing"
)

func TestToolchainDirective(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		version, err := ToolchainDirective([]byte("module example.com/tenant\n\ngo 1.26\n\ntoolchain go1.26.6\n"))
		if err != nil {
			t.Fatalf("ToolchainDirective: %v", err)
		}
		if version != "go1.26.6" {
			t.Fatalf("version = %q", version)
		}
	})

	t.Run("short form", func(t *testing.T) {
		version, err := ToolchainDirective([]byte("module m\n\ntoolchain go1.26\n"))
		if err != nil {
			t.Fatalf("ToolchainDirective: %v", err)
		}
		if version != "go1.26" {
			t.Fatalf("version = %q", version)
		}
	})

	t.Run("crlf and comments", func(t *testing.T) {
		version, err := ToolchainDirective([]byte("module m\r\n\r\n// toolchain go1.25.0 is a comment, not the directive\r\ntoolchain go1.26.6 // pinned\r\n"))
		if err != nil {
			t.Fatalf("ToolchainDirective: %v", err)
		}
		if version != "go1.26.6" {
			t.Fatalf("version = %q", version)
		}
	})

	t.Run("empty", func(t *testing.T) {
		if _, err := ToolchainDirective(nil); err == nil {
			t.Fatal("expected the empty rejection")
		}
	})

	t.Run("absent", func(t *testing.T) {
		_, err := ToolchainDirective([]byte("module m\n\ngo 1.26\n"))
		if err == nil || !strings.Contains(err.Error(), "no toolchain directive") {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("malformed", func(t *testing.T) {
		_, err := ToolchainDirective([]byte("module m\n\ntoolchain latest\n"))
		if err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestVerifyToolchain(t *testing.T) {
	t.Run("pass", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) {
				return []byte("module example.com/tenant\n\ntoolchain go1.26.6\n"), nil
			},
		}
		if findings := verifier.verifyToolchain(); len(findings) != 0 {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("read error", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) { return nil, errors.New("boom") },
		}
		findings := verifier.verifyToolchain()
		if len(findings) != 1 || !strings.Contains(findings[0].Detail, "boom") {
			t.Fatalf("findings = %v", findings)
		}
	})

	t.Run("missing directive", func(t *testing.T) {
		verifier := Verifier{
			ReadTenant: func(path string) ([]byte, error) { return []byte("module example.com/tenant\n"), nil },
		}
		findings := verifier.verifyToolchain()
		if len(findings) != 1 || !strings.Contains(findings[0].Detail, "no toolchain directive") {
			t.Fatalf("findings = %v", findings)
		}
	})
}
