package canonical

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// qualityConfigDocument is the structural wire form of the config seam
// (git-governance.quality.json) at schema version 4. The verifier proves the
// pinned schema version with strict structural decoding; the semantic gate
// rules (gate-name uniqueness, duration parsing, discovery overrides) are
// owned by the producer home that executes the gate — the tenant's quality
// lane runs that decoder as a required check.
type qualityConfigDocument struct {
	SchemaVersion int                  `json:"schemaVersion"`
	Toolchain     qualityToolchainJSON `json:"toolchain"`
	Extends       []string             `json:"extends"`
	Defaults      qualityScopeJSON     `json:"defaults"`
	Gates         []qualityGateJSON    `json:"gates"`
	Project       qualityProjectJSON   `json:"project"`
}

// qualityToolchainJSON is the language-keyed toolchain identity of the v4
// seam: one form serves every language territory without a schema fork.
type qualityToolchainJSON struct {
	Language string `json:"language"`
	Version  string `json:"version"`
}

type qualityScopeJSON struct {
	IncludeFamilies []string `json:"includeFamilies"`
	ExcludeFamilies []string `json:"excludeFamilies"`
}

type qualityGateJSON struct {
	Name             string   `json:"name"`
	Command          string   `json:"command"`
	Args             []string `json:"args"`
	Timeout          string   `json:"timeout"`
	WorkingDirectory string   `json:"workingDirectory"`
	IncludeFamilies  []string `json:"includeFamilies"`
	ExcludeFamilies  []string `json:"excludeFamilies"`
}

type qualityProjectJSON struct {
	Binaries []qualityBinaryJSON `json:"binaries"`
	Fuzz     []qualityFuzzJSON   `json:"fuzz"`
}

type qualityBinaryJSON struct {
	Package string   `json:"package"`
	Smoke   []string `json:"smoke"`
}

type qualityFuzzJSON struct {
	Package string `json:"package"`
	Target  string `json:"target"`
	Time    string `json:"time"`
}

// decodeQualityConfig strictly decodes the tenant's configuration seam wire
// form. Unknown fields, trailing documents, and type mismatches are rejected
// with a precise error.
func decodeQualityConfig(contents []byte) (qualityConfigDocument, error) {
	var document qualityConfigDocument
	if len(contents) == 0 {
		return document, errors.New("quality configuration must not be empty")
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return document, fmt.Errorf("quality configuration must contain valid JSON with known fields: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return document, errors.New("quality configuration must contain exactly one JSON document")
	}
	return document, nil
}

// DecodeQualityConfigVersion strictly decodes the tenant's configuration seam
// and returns its declared schema version.
func DecodeQualityConfigVersion(contents []byte) (int, error) {
	document, err := decodeQualityConfig(contents)
	if err != nil {
		return 0, err
	}
	return document.SchemaVersion, nil
}

// DecodeQualityConfigExtends strictly decodes the tenant's configuration seam
// and returns its declared capability pack references. The reference grammar
// is validated by the producer home's decoder at gate time; the verifier
// proves the resolution of every declared reference fail-closed.
func DecodeQualityConfigExtends(contents []byte) ([]string, error) {
	document, err := decodeQualityConfig(contents)
	if err != nil {
		return nil, err
	}
	return document.Extends, nil
}

// verifyQuality proves the tenant's configuration seam strictly decodes and
// carries the pinned schema version.
func (v Verifier) verifyQuality(bindings Bindings) []Finding {
	check := "quality config"
	contents, err := v.ReadTenant(bindings.Quality.Config)
	if err != nil {
		return []Finding{readErrorFinding(check, bindings.Quality.Config, err)}
	}
	version, err := DecodeQualityConfigVersion(contents)
	if err != nil {
		return []Finding{mismatchFinding(check, err.Error())}
	}
	if version != bindings.Quality.SchemaVersion {
		return []Finding{mismatchFinding(check,
			fmt.Sprintf("the configuration seam declares schemaVersion %d, but the binding pins %d", version, bindings.Quality.SchemaVersion))}
	}
	return nil
}
