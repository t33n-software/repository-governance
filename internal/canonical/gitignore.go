package canonical

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// gitignoreTreeRoot is the home-relative root of the gitignore fragment tree.
const gitignoreTreeRoot = "hosting-platforms/github/files/gitignore"

// projectBlockMark terminates the governed region of a composed tenant
// gitignore file; the tenant's free project block lives below it.
const projectBlockMark = "# -- project additions below this line --"

// gitignoreFragmentNamePattern binds the fragment naming grammar: kebab-case
// segments with at most one slash — the org core ("core"), an ecosystem area
// ("<area>/core"), or a concern fragment ("<area>/<concern>").
var gitignoreFragmentNamePattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*(?:/[a-z0-9]+(?:-[a-z0-9]+)*)?$`)

// canonicalSecretFamilies is the home's projection of the universal
// secret-artifact guard families owned by the push-protection governance; the
// org core fragment must carry every family.
var canonicalSecretFamilies = []string{
	".env", ".env.*", ".envrc", ".env*.local",
	"credentials", "credentials.*",
	"*.pem", "*.key", "*.p12", "*.pfx", "*.jks", "*.keystore", "*.kdbx", "*.ppk", "*.gpg",
}

// licenseFamilyProbes are the canonical probe paths of the protected license
// family: the two root files, the directory itself (Git's parent-exclusion
// rule makes a content path unverifiable once its directory is excluded), and
// one content path below it.
var licenseFamilyProbes = []struct {
	path  []string
	isDir bool
	name  string
}{
	{path: []string{"LICENSE"}, isDir: false, name: "LICENSE"},
	{path: []string{"NOTICE"}, isDir: false, name: "NOTICE"},
	{path: []string{"LICENSES"}, isDir: true, name: "LICENSES/"},
	{path: []string{"LICENSES", "placeholder"}, isDir: false, name: "LICENSES/<content>"},
}

// ValidateGitignoreFragments binds the fragment list form: the org core
// first, every name in the fragment grammar, and no duplicates.
func ValidateGitignoreFragments(fragments []string) error {
	if len(fragments) == 0 {
		return errors.New("must contain at least the core fragment")
	}
	if fragments[0] != "core" {
		return fmt.Errorf("must begin with the core fragment, got %q", fragments[0])
	}
	seen := make(map[string]struct{}, len(fragments))
	for _, fragment := range fragments {
		if !gitignoreFragmentNamePattern.MatchString(fragment) {
			return fmt.Errorf("must be a canonical fragment name, got %q", fragment)
		}
		if !strings.Contains(fragment, "/") && fragment != "core" {
			return fmt.Errorf("must name an area or concern below the org core, got %q", fragment)
		}
		if _, found := seen[fragment]; found {
			return fmt.Errorf("must not repeat a fragment, got %q", fragment)
		}
		seen[fragment] = struct{}{}
	}
	return nil
}

// RenderGitignoreGovernedRegion composes the governed region of a tenant
// gitignore file from the bound fragment list read from the home tree: the
// generated header naming the source fragments and the home pin, each
// fragment's content with its generated source header, and exactly one
// project-block mark at the end. The render is fail-closed: an invalid
// fragment list, an unreadable fragment, a mark inside a fragment, a
// cross-fragment pattern restatement, a negation in the org core, a missing
// secret-artifact family, or a license-family match inside the composed
// region is an error, never a weakened output.
func RenderGitignoreGovernedRegion(readHome func(path string) ([]byte, error), fragments []string, homeSHA string) ([]byte, error) {
	if err := ValidateGitignoreFragments(fragments); err != nil {
		return nil, fmt.Errorf("the fragment list %s", err)
	}
	type fragmentContent struct {
		name     string
		content  string
		patterns []string
	}
	contents := make([]fragmentContent, 0, len(fragments))
	seenPatterns := make(map[string]string)
	for _, fragment := range fragments {
		path := gitignoreTreeRoot + "/" + fragment + ".gitignore"
		raw, err := readHome(path)
		if err != nil {
			return nil, fmt.Errorf("read the fragment %s: %w", fragment, err)
		}
		content := string(raw)
		if strings.Contains(content, projectBlockMark) {
			return nil, fmt.Errorf("the fragment %s carries the project-block mark", fragment)
		}
		patterns := gitignorePatternLines(content)
		for _, pattern := range patterns {
			normalized := strings.TrimSpace(pattern)
			if owner, found := seenPatterns[normalized]; found {
				return nil, fmt.Errorf("the fragment %s restates the pattern %q of %s", fragment, normalized, owner)
			}
			seenPatterns[normalized] = fragment
		}
		contents = append(contents, fragmentContent{name: fragment, content: strings.TrimRight(content, "\n"), patterns: patterns})
	}

	core := contents[0]
	corePatterns := make(map[string]struct{}, len(core.patterns))
	for _, pattern := range core.patterns {
		if strings.HasPrefix(strings.TrimSpace(pattern), "!") {
			return nil, fmt.Errorf("the org core carries the negation %q, which is no protection mechanism", strings.TrimSpace(pattern))
		}
		corePatterns[strings.TrimSpace(pattern)] = struct{}{}
	}
	for _, family := range canonicalSecretFamilies {
		if _, found := corePatterns[family]; !found {
			return nil, fmt.Errorf("the org core misses the secret-artifact family %q", family)
		}
	}

	var region strings.Builder
	region.WriteString("# canonical: gitignore " + strings.Join(fragments, " + ") + " @ " + homeSHA + " — governed region, do not edit\n")
	region.WriteString(core.content)
	for _, fragment := range contents[1:] {
		region.WriteString("\n\n# " + fragment.name + "\n")
		region.WriteString(fragment.content)
	}
	region.WriteString("\n\n" + projectBlockMark + "\n")

	rendered := region.String()
	if violations := GitignoreLicenseViolations([]byte(rendered)); len(violations) > 0 {
		return nil, fmt.Errorf("the governed region ignores the protected license family: %s", strings.Join(violations, ", "))
	}
	return []byte(rendered), nil
}

// gitignorePatternLines extracts the pattern lines of a gitignore document:
// neither blank nor comment lines. The lines keep their raw form for the
// matching engine; only the carriage-return artifact of Windows line endings
// is stripped.
func gitignorePatternLines(content string) []string {
	patterns := make([]string, 0)
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		patterns = append(patterns, strings.TrimRight(line, "\r"))
	}
	return patterns
}

// GitignoreLicenseViolations evaluates a composed gitignore document against
// the canonical license-family probes with the Git-compatible pattern
// semantics of the bound engine and returns the ignored probe names; an empty
// result is the only pass.
func GitignoreLicenseViolations(content []byte) []string {
	patterns := make([]gitignore.Pattern, 0)
	for _, line := range gitignorePatternLines(string(content)) {
		patterns = append(patterns, gitignore.ParsePattern(line, nil))
	}
	matcher := gitignore.NewMatcher(patterns)
	violations := make([]string, 0)
	for _, probe := range licenseFamilyProbes {
		if matcher.Match(probe.path, probe.isDir) {
			violations = append(violations, probe.name)
		}
	}
	return violations
}
