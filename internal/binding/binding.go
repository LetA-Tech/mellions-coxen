// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package binding reads the keys Mellions acts on from a repository's
// .claude/repo-binding.yaml.
//
// Each key is read with one pattern rather than a YAML dependency. An absent
// key and a shape the pattern does not match give the same answer, "not
// declared", and every caller reads the file through this one parser so a lane
// and the survey cannot disagree about what a repository declares.
package binding

import "regexp"

// Path is where a repository keeps its binding, relative to its root.
const Path = ".claude/repo-binding.yaml"

// NotDeclared is the model reported when approval authorities are named and
// approval_model is not. What that defaults to is the repository's proposal
// method to say, not this parser's.
const NotDeclared = "not declared"

// Binding is what one repo-binding.yaml declares.
type Binding struct {
	// DevelopmentBranch is development_branch, or "" when not declared.
	DevelopmentBranch string
	// Authorities is approval_authorities as written, e.g. "[lucas, antony]",
	// or "" when the file names none.
	Authorities string
	// Model is approval_model, NotDeclared when authorities are named without
	// one, and "" when no authorities are named.
	Model string
}

var (
	developmentBranchKey   = regexp.MustCompile(`(?m)^development_branch:[ \t]*["']?([A-Za-z0-9._/-]+)`)
	approvalAuthoritiesKey = regexp.MustCompile(`(?m)^approval_authorities:[ \t]*(\[[^\]\n]*\])`)
	approvalModelKey       = regexp.MustCompile(`(?m)^[ \t]*approval_model:[ \t]*["']?([A-Za-z0-9_-]+)`)
)

// Parse reads the binding's keys from raw file content.
func Parse(raw []byte) Binding {
	var b Binding
	if m := developmentBranchKey.FindSubmatch(raw); m != nil {
		b.DevelopmentBranch = string(m[1])
	}
	m := approvalAuthoritiesKey.FindSubmatch(raw)
	if m == nil {
		return b
	}
	b.Authorities = string(m[1])
	b.Model = NotDeclared
	if mm := approvalModelKey.FindSubmatch(raw); mm != nil {
		b.Model = string(mm[1])
	}
	return b
}
