package canonical

import (
	"fmt"
	"strings"
)

// conventionsTemplatePath is the home-relative path of the rule-sets
// conventions README template.
const conventionsTemplatePath = "hosting-platforms/github/files/conventions/rule-sets-readme.md.tmpl"

// The render tokens the conventions template carries.
const (
	conventionsOrganizationToken = "{{organization}}"
	conventionsRepositoryToken   = "{{repository}}"
	conventionsClassToken        = "{{class}}"
	conventionsPlatformsToken    = "{{platforms}}"
	conventionsRationaleToken    = "{{rationale}}"
)

// conventionsClassPlatforms maps every quality-gates class with a canonical
// render to its platform sentence; a class without a canonical render fails
// closed.
var conventionsClassPlatforms = map[string]string{
	"full":       "The quality gates run on **Linux**, **Windows**, and **macOS**.",
	"linux-only": "The quality gates run exclusively on **Linux**.",
}

// verifyConventions proves the conventions family where bound: the tenant's
// rule-sets README is the exact materialization of the canonical template
// with the manifest's values and the class-derived platform sentence. A
// tenant without the conventions binding skips the proof; a bound tenant
// fails closed on any divergence.
func (v Verifier) verifyConventions(bindings Bindings) []Finding {
	conventions := bindings.Conventions
	if conventions == nil {
		return nil
	}
	check := "conventions"
	templateContents, err := v.ReadHome(conventionsTemplatePath)
	if err != nil {
		return []Finding{readErrorFinding(check, conventionsTemplatePath, err)}
	}
	rendered, err := renderConventionsReadme(string(templateContents), *conventions, bindings.Class.QualityGates)
	if err != nil {
		return []Finding{mismatchFinding(check, err.Error())}
	}
	tenantContents, err := v.ReadTenant(conventions.Path)
	if err != nil {
		return []Finding{readErrorFinding(check, conventions.Path, err)}
	}
	if string(tenantContents) != rendered {
		return []Finding{mismatchFinding(check, fmt.Sprintf(
			"the tenant %s is not the materialization of the canonical template with the bound values", conventions.Path))}
	}
	return nil
}

// renderConventionsReadme renders the canonical rule-sets README template
// with the bound values: every token must be present in the template, and
// the class must carry a canonical platform sentence.
func renderConventionsReadme(template string, conventions ConventionsBinding, class string) (string, error) {
	platforms, found := conventionsClassPlatforms[class]
	if !found {
		return "", fmt.Errorf("the quality-gates class %q carries no canonical conventions render", class)
	}
	replacements := []struct{ token, value string }{
		{conventionsOrganizationToken, conventions.Organization},
		{conventionsRepositoryToken, conventions.Repository},
		{conventionsClassToken, class},
		{conventionsPlatformsToken, platforms},
		{conventionsRationaleToken, conventions.Rationale},
	}
	rendered := template
	for _, replacement := range replacements {
		if !strings.Contains(rendered, replacement.token) {
			return "", fmt.Errorf("the canonical template carries no %s token", replacement.token)
		}
		rendered = strings.ReplaceAll(rendered, replacement.token, replacement.value)
	}
	return rendered, nil
}
