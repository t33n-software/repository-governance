package canonical

import (
	"fmt"
	"strings"
)

// codeownersTemplatePath is the home-relative path of the ownership template.
const codeownersTemplatePath = "files/codeowners/CODEOWNERS.tmpl"

// codeownersToken is the render token the template carries for the default
// owner value.
const codeownersToken = "{{defaultOwner}}"

// fileTopics maps each canonical file topic to its home-relative master path.
var fileTopics = []struct {
	topic     string
	homePath  string
	binding   func(FileBindings) FileBinding
	prefixMode bool
}{
	{topic: "lefthook", homePath: "files/lefthook/lefthook.yml", binding: func(f FileBindings) FileBinding { return f.Lefthook }},
	{topic: "gitattributes", homePath: "files/gitattributes/.gitattributes", binding: func(f FileBindings) FileBinding { return f.Gitattributes }},
	{topic: "gitignore", homePath: "files/gitignore/.gitignore", binding: func(f FileBindings) FileBinding { return f.Gitignore }, prefixMode: true},
	{topic: "dependabot", homePath: "files/dependabot/dependabot-go.yml", binding: func(f FileBindings) FileBinding { return f.Dependabot }},
}

// verifyFiles proves the canonical file family: byte-identical topics compare
// by hash against both the home master and the declared hash; the gitignore
// topic proves the canonical core as a verbatim prefix of the tenant file.
func (v Verifier) verifyFiles(bindings Bindings) []Finding {
	findings := make([]Finding, 0)
	for _, topic := range fileTopics {
		findings = append(findings, v.verifyFileTopic(bindings.Files, topic.topic, topic.homePath, topic.binding, topic.prefixMode)...)
	}
	return findings
}

// verifyFileTopic runs the proofs of one canonical file topic.
func (v Verifier) verifyFileTopic(files FileBindings, topic, homePath string, binding func(FileBindings) FileBinding, prefixMode bool) []Finding {
	check := "file " + topic
	declared := binding(files)

	homeContents, err := v.ReadHome(homePath)
	if err != nil {
		return []Finding{readErrorFinding(check, homePath, err)}
	}
	if hash := Sum256Hex(homeContents); hash != declared.SHA256 {
		return []Finding{mismatchFinding(check,
			fmt.Sprintf("the canonical %s hash %s diverges from the bound hash %s", topic, hash, declared.SHA256))}
	}

	tenantContents, err := v.ReadTenant(declared.Path)
	if err != nil {
		return []Finding{readErrorFinding(check, declared.Path, err)}
	}
	if prefixMode {
		if !strings.HasPrefix(string(tenantContents), string(homeContents)) {
			return []Finding{mismatchFinding(check,
				fmt.Sprintf("the tenant %s does not carry the canonical core as a verbatim prefix", declared.Path))}
		}
		return nil
	}
	if hash := Sum256Hex(tenantContents); hash != declared.SHA256 {
		return []Finding{mismatchFinding(check,
			fmt.Sprintf("the tenant %s hash %s diverges from the bound hash %s", declared.Path, hash, declared.SHA256))}
	}
	return nil
}

// verifyCodeowners proves the tenant's ownership file is the exact render of
// the canonical template with the manifest's values.
func (v Verifier) verifyCodeowners(bindings Bindings) []Finding {
	check := "codeowners"
	templateContents, err := v.ReadHome(codeownersTemplatePath)
	if err != nil {
		return []Finding{readErrorFinding(check, codeownersTemplatePath, err)}
	}
	template := string(templateContents)
	if !strings.Contains(template, codeownersToken) {
		return []Finding{mismatchFinding(check,
			fmt.Sprintf("the canonical template carries no %s token", codeownersToken))}
	}
	rendered := strings.ReplaceAll(template, codeownersToken, bindings.Codeowners.DefaultOwner)
	tenantContents, err := v.ReadTenant(bindings.Codeowners.Path)
	if err != nil {
		return []Finding{readErrorFinding(check, bindings.Codeowners.Path, err)}
	}
	if string(tenantContents) != rendered {
		return []Finding{mismatchFinding(check,
			fmt.Sprintf("the tenant %s is not the materialization of the canonical template with the bound values", bindings.Codeowners.Path))}
	}
	return nil
}
