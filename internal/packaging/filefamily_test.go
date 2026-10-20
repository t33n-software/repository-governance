package packaging

import (
	"strings"
	"testing"
)

func TestCanonicalFileFamily(t *testing.T) {
	gitattributes := readArtifact(t, "hosting-platforms/github/files/gitattributes/.gitattributes")
	if gitattributes != "* text=auto eol=lf\n"+
		"*.7z filter=lfs diff=lfs merge=lfs -text\n"+
		"*.duckdb filter=lfs diff=lfs merge=lfs -text\n"+
		"*.gguf filter=lfs diff=lfs merge=lfs -text\n"+
		"*.onnx filter=lfs diff=lfs merge=lfs -text\n"+
		"*.parquet filter=lfs diff=lfs merge=lfs -text\n"+
		"*.pdf filter=lfs diff=lfs merge=lfs -text\n"+
		"*.pt filter=lfs diff=lfs merge=lfs -text\n"+
		"*.safetensors filter=lfs diff=lfs merge=lfs -text\n"+
		"*.sqlite filter=lfs diff=lfs merge=lfs -text\n"+
		"*.sqlite3 filter=lfs diff=lfs merge=lfs -text\n"+
		"*.tar filter=lfs diff=lfs merge=lfs -text\n"+
		"*.tar.gz filter=lfs diff=lfs merge=lfs -text\n"+
		"*.tgz filter=lfs diff=lfs merge=lfs -text\n"+
		"*.zip filter=lfs diff=lfs merge=lfs -text\n" {
		t.Fatalf("the gitattributes core drifted: %q", gitattributes)
	}

	lefthook := readArtifact(t, "hosting-platforms/github/files/lefthook/lefthook.yml")
	for _, required := range []string{
		"git-governance --interactive never commit validate --message-file",
		"git-governance --interactive never validate pre-push --remote",
	} {
		if !strings.Contains(lefthook, required) {
			t.Fatalf("the lefthook core must call the Git CLI: %q", required)
		}
	}

	dependabot := readArtifact(t, "hosting-platforms/github/files/dependabot/dependabot-go.yml")
	for _, ecosystem := range []string{"package-ecosystem: gomod", "package-ecosystem: github-actions", "target-branch: develop"} {
		if !strings.Contains(dependabot, ecosystem) {
			t.Fatalf("the dependabot variant must carry %q", ecosystem)
		}
	}

	dependabotNode := readArtifact(t, "hosting-platforms/github/files/dependabot/dependabot-node.yml")
	for _, ecosystem := range []string{"package-ecosystem: npm", "package-ecosystem: github-actions", "target-branch: develop"} {
		if !strings.Contains(dependabotNode, ecosystem) {
			t.Fatalf("the dependabot node variant must carry %q", ecosystem)
		}
	}
}

func TestCodeownersTemplateAndMaterialization(t *testing.T) {
	template := readArtifact(t, "hosting-platforms/github/files/codeowners/CODEOWNERS.tmpl")
	if !strings.Contains(template, "{{defaultOwner}}") {
		t.Fatal("the CODEOWNERS template must carry the defaultOwner token")
	}
	rendered := strings.ReplaceAll(template, "{{defaultOwner}}", "@CyberT33N")
	if own := readArtifact(t, ".github/CODEOWNERS"); own != rendered {
		t.Fatalf("the home's own CODEOWNERS is not the materialization of the template:\n%s", own)
	}
}

// TestConventionsTemplate proves the canonical rule-sets conventions README
// template: the full token surface, the canonical section structure, LF-only
// bytes, and value freedom — the template never carries an organization,
// repository, class, or rationale value.
func TestConventionsTemplate(t *testing.T) {
	template := readArtifact(t, "hosting-platforms/github/files/conventions/rule-sets-readme.md.tmpl")
	for _, token := range []string{"{{organization}}", "{{repository}}", "{{class}}", "{{platforms}}", "{{rationale}}"} {
		if !strings.Contains(template, token) {
			t.Fatalf("the conventions template must carry the %s token", token)
		}
	}
	for _, section := range []string{"## Canonical source", "## Family in use", "## Bound rule sets", "## Management"} {
		if !strings.Contains(template, section) {
			t.Fatalf("the conventions template must carry the section %q", section)
		}
	}
	if strings.Contains(template, "\r") {
		t.Fatal("the conventions template must be LF-only")
	}
	if strings.Contains(template, "t33n") {
		t.Fatal("the conventions template must stay organization-agnostic")
	}
}
