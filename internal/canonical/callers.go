package canonical

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// callerHashesDocument is the wire form of the home's published caller-hash
// record (caller-hashes/v1).
type callerHashesDocument struct {
	SchemaVersion int `json:"schemaVersion"`
	Home          struct {
		Repository string `json:"repository"`
		Version    string `json:"version"`
		SHA        string `json:"sha"`
	} `json:"home"`
	Callers []struct {
		Master     string `json:"master"`
		TenantFile string `json:"tenantFile"`
		Class      string `json:"class"`
		SHA256     string `json:"sha256"`
	} `json:"callers"`
}

// callerHashesPath is the home-relative path of the published record.
const callerHashesPath = "hosting-platforms/github/workflows/callers/go/caller-hashes.json"

// verifyCallers proves every bound caller: the tenant file's hash equals the
// declared hash, the canonical master's hash equals the declared hash, the
// home's published record carries the declared hash, and the tenant file
// references the payload by the bound home pin.
func (v Verifier) verifyCallers(bindings Bindings) []Finding {
	findings := make([]Finding, 0)
	published, err := v.publishedCallerHashes()
	if err != nil {
		return []Finding{readErrorFinding("callers", callerHashesPath, err)}
	}
	for _, caller := range bindings.Callers {
		check := "caller " + caller.File
		findings = append(findings, v.verifyCallerHash(bindings, caller, published, check)...)
	}
	return findings
}

// verifyCallerHash runs the four hash and pin proofs of one caller.
func (v Verifier) verifyCallerHash(bindings Bindings, caller CallerBinding, published map[string]string, check string) []Finding {
	findings := make([]Finding, 0)

	tenantContents, err := v.ReadTenant(caller.File)
	if err != nil {
		return append(findings, readErrorFinding(check, caller.File, err))
	}
	if hash := Sum256Hex(tenantContents); hash != caller.SHA256 {
		findings = append(findings, mismatchFinding(check,
			fmt.Sprintf("the tenant caller hash %s diverges from the bound hash %s", hash, caller.SHA256)))
	}

	masterContents, err := v.ReadHome(caller.Master)
	if err != nil {
		findings = append(findings, readErrorFinding(check, caller.Master, err))
	} else if hash := Sum256Hex(masterContents); hash != caller.SHA256 {
		findings = append(findings, mismatchFinding(check,
			fmt.Sprintf("the canonical master hash %s diverges from the bound hash %s", hash, caller.SHA256)))
	}

	if recorded, found := published[caller.Master]; !found {
		findings = append(findings, mismatchFinding(check,
			fmt.Sprintf("the home's published caller-hashes record carries no entry for %s", caller.Master)))
	} else if recorded != caller.SHA256 {
		findings = append(findings, mismatchFinding(check,
			fmt.Sprintf("the published caller hash %s diverges from the bound hash %s", recorded, caller.SHA256)))
	}

	if err := verifyCallerPin(string(tenantContents), bindings.Home); err != nil {
		findings = append(findings, mismatchFinding(check, err.Error()))
	}
	return findings
}

// verifyCallerPin proves the tenant caller references the home payload by the
// bound full-length SHA.
func verifyCallerPin(contents string, home HomePin) error {
	prefix := home.Repository + "/.github/workflows/reusable-"
	for _, line := range strings.Split(strings.ReplaceAll(contents, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		uses, found := strings.CutPrefix(trimmed, "uses: ")
		if !found {
			continue
		}
		coordinate, reference, referenceFound := strings.Cut(uses, "@")
		if !referenceFound {
			return fmt.Errorf("the caller reference %q carries no pin", uses)
		}
		if !strings.HasPrefix(coordinate, prefix) {
			return fmt.Errorf("the caller references %q, not the bound home %q", coordinate, home.Repository)
		}
		if reference != home.SHA {
			return fmt.Errorf("the caller pin %s diverges from the bound home SHA %s", reference, home.SHA)
		}
		return nil
	}
	return fmt.Errorf("the caller carries no uses reference")
}

// publishedCallerHashes reads the home's published caller-hash record.
func (v Verifier) publishedCallerHashes() (map[string]string, error) {
	contents, err := v.ReadHome(callerHashesPath)
	if err != nil {
		return nil, err
	}
	document, err := decodeCallerHashes(contents)
	if err != nil {
		return nil, err
	}
	record := make(map[string]string, len(document.Callers))
	for _, caller := range document.Callers {
		record[caller.Master] = caller.SHA256
	}
	return record, nil
}

// decodeCallerHashes strictly decodes the home's caller-hashes/v1 record.
func decodeCallerHashes(contents []byte) (callerHashesDocument, error) {
	if len(contents) == 0 {
		return callerHashesDocument{}, errors.New("the caller-hashes record must not be empty")
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var document callerHashesDocument
	if err := decoder.Decode(&document); err != nil {
		return callerHashesDocument{}, fmt.Errorf("the caller-hashes record must contain valid JSON with known fields: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return callerHashesDocument{}, errors.New("the caller-hashes record must contain exactly one JSON document")
	}
	if document.SchemaVersion != 1 {
		return callerHashesDocument{}, fmt.Errorf("the caller-hashes record schemaVersion must equal %d", 1)
	}
	if len(document.Callers) == 0 {
		return callerHashesDocument{}, errors.New("the caller-hashes record must carry at least one caller")
	}
	seen := make(map[string]struct{}, len(document.Callers))
	for _, caller := range document.Callers {
		if caller.Master == "" || caller.TenantFile == "" {
			return callerHashesDocument{}, errors.New("every caller-hashes entry must carry master and tenantFile")
		}
		if !hashPattern.MatchString(caller.SHA256) {
			return callerHashesDocument{}, fmt.Errorf("the caller-hashes entry for %q must carry a lowercase SHA-256 hex digest", caller.Master)
		}
		if _, found := seen[caller.Master]; found {
			return callerHashesDocument{}, fmt.Errorf("the caller-hashes record carries %q more than once", caller.Master)
		}
		seen[caller.Master] = struct{}{}
	}
	return document, nil
}
