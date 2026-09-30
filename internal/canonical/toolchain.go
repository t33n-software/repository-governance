package canonical

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// toolchainDirectivePattern is the well-formed form of a go.mod toolchain
// directive (the Go-native toolchain selector).
var toolchainDirectivePattern = regexp.MustCompile(`^toolchain go[0-9]+\.[0-9]+(\.[0-9]+)?$`)

// ToolchainDirective extracts the toolchain directive of a go.mod declaration
// — the Go-native selector the home's payloads and composite actions resolve
// fail-closed and install exactly through setup-go's go-version input. The
// directive must exist and be well-formed; a missing or malformed directive
// is a fail-closed error.
func ToolchainDirective(contents []byte) (string, error) {
	if len(contents) == 0 {
		return "", errors.New("the go.mod declaration must not be empty")
	}
	for _, raw := range strings.Split(strings.ReplaceAll(string(contents), "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if index := strings.Index(line, "//"); index >= 0 {
			line = strings.TrimSpace(line[:index])
		}
		if line == "" {
			continue
		}
		directive, found := strings.CutPrefix(line, "toolchain ")
		if !found {
			continue
		}
		if !toolchainDirectivePattern.MatchString(line) {
			return "", fmt.Errorf("the go.mod toolchain directive is malformed: %q", line)
		}
		return directive, nil
	}
	return "", errors.New("the go.mod declaration carries no toolchain directive")
}

// verifyToolchain proves the tenant's Go module declaration carries an
// explicit, well-formed toolchain directive — the Go-native selector the
// payloads resolve fail-closed and install exactly. The version cross-check
// against the configuration seam's toolchain version is owned by the quality
// gate at runtime.
func (v Verifier) verifyToolchain() []Finding {
	check := "toolchain directive"
	contents, err := v.ReadTenant("go.mod")
	if err != nil {
		return []Finding{readErrorFinding(check, "go.mod", err)}
	}
	if _, err := ToolchainDirective(contents); err != nil {
		return []Finding{mismatchFinding(check, err.Error())}
	}
	return nil
}
