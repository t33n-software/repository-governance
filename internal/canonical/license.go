package canonical

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// licenseBindings are the files the license-hub family wires into a tenant.
var licenseBindings = []string{"license.values.json", "license.lock.json"}

// verifyLicense proves the license-lane wiring where the license-hub family
// is bound: the binding values and lock exist and decode as JSON documents.
// The license content verification itself is owned by the license-hub's own
// tool; this proof covers the wiring, never the payload.
func (v Verifier) verifyLicense(bindings Bindings) []Finding {
	if !bindings.Class.LicenseHub {
		return nil
	}
	findings := make([]Finding, 0)
	for _, path := range licenseBindings {
		contents, err := v.ReadTenant(path)
		if err != nil {
			findings = append(findings, readErrorFinding("license", path, err))
			continue
		}
		if err := decodeJSONDocument(contents); err != nil {
			findings = append(findings, mismatchFinding("license",
				path+" must contain a valid JSON document"))
		}
	}
	return findings
}

// decodeJSONDocument proves the contents hold exactly one valid JSON value.
func decodeJSONDocument(contents []byte) error {
	if len(contents) == 0 {
		return errors.New("empty document")
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	var document any
	if err := decoder.Decode(&document); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("more than one JSON document")
	}
	return nil
}
