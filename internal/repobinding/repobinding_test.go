// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package repobinding

import "testing"

func TestParse(t *testing.T) {
	for _, c := range []struct {
		name, raw string
		want      Binding
	}{
		{"empty", "", Binding{}},
		{"branch only", "development_branch: dev\n", Binding{DevelopmentBranch: "dev"}},
		{"model without authorities is no gate", "approval_model: merge-gate\n", Binding{}},
		{"authorities without model", "approval_authorities: [lucas]\n",
			Binding{Authorities: "[lucas]", Model: NotDeclared}},
		{"both, model nested and quoted", "development_branch: \"main\"\napproval_authorities: [lucas, antony]\nproposal:\n  approval_model: 'pre-proposal'\n",
			Binding{DevelopmentBranch: "main", Authorities: "[lucas, antony]", Model: "pre-proposal"}},
		{"indented authorities are not the top-level key", "  approval_authorities: [x]\n", Binding{}},
	} {
		if got := Parse([]byte(c.raw)); got != c.want {
			t.Errorf("%s: Parse = %+v, want %+v", c.name, got, c.want)
		}
	}
}
