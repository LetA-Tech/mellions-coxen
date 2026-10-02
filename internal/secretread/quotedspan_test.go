// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package secretread

import "testing"

// A regex or sed script quoted with whitespace inside a substitution is one
// argument, not a dotfile glob; every substitution that can print a credential
// through a glob — the shell's, a program's own, or a re-parsed script's —
// stays refused.
func TestScanBash_QuotedRegexInsideSubstitution(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want string // the path the denial names; "" is allowed
	}{
		// The two denials one unattended shift met on the Value path.
		{"git grep regex with a space", "m=$(git -C x grep -nE 'install .*protoc-gen-go(-grpc)?@' ref -- Makefile)\necho \"$m\"", ""},
		{"sed script with a space", "o=$(sed -n \"3p\" d/Makefile | sed 's/.*--go-grpc_out=[^ ]* //; s/[;)].*//')\necho $o", ""},
		{"printed directly", `echo "$(grep -E 'a .*b' notes.txt)"`, ""},
		{"backtick form", "echo `sed 's/.*= //' version.txt`", ""},
		{"double-quoted regex", `echo "$(grep -E "a .*b" notes.txt)"`, ""},

		// Unquoted globs the shell expands.
		{"unquoted dot glob", "x=$(cat .*)\necho $x", ".*"},
		{"unquoted env glob", "x=$(cat .env*)\necho $x", ".env*"},
		{"unquoted beside a quoted regex", "x=$(grep 'a .*b' .*)\necho $x", ".*"},
		{"tail of the name", "x=$(tail -1 .db_connection)\necho $x", ".db_connection"},
		{"quoted name", "x=$(cat '.env')\necho $x", ".env"},

		// Quoted globs a program expands itself: one argument, no whitespace.
		{"find -name", "x=$(find . -name '.db_conn*' -exec cat {} +)\necho $x", ".db_conn*"},
		{"find -name dot", "x=$(find . -name '.*' -exec cat {} +)\necho $x", ".*"},
		{"git pathspec", "x=$(git grep -h K -- '.db_conn*')\necho $x", ".db_conn*"},

		// A quoted script something re-parses, where `.*` is a glob again.
		{"bash -c", "x=$(bash -c 'cat .* | head')\necho $x", ".*"},
		{"sh -c via xargs", "x=$(echo . | xargs sh -c 'cat .* 2>/dev/null')\necho $x", ".*"},
		{"eval", "x=$(eval 'cat .* ')\necho $x", ".*"},
		{"awk system", "x=$(awk 'BEGIN{system(\"cat .* \")}')\necho $x", ".*"},
		{"python3 glob", "x=$(python3 -c \"import glob; print(glob.glob('.* x'))\")\necho $x", ".*"},
		{"ssh remote shell", "x=$(ssh h 'cat .* ')\necho $x", ".*"},
		{"a quoted name in a script", "x=$(python3 -c \"import os; print(open('.env').read())\")\necho $x", ".env"},

		// Quoting that does not balance exempts nothing.
		{"case pattern paren", "x=\"$(case a in a) cat .* ;; esac)\"\necho $x", ".*"},
		{"case pattern paren, unquoted", "x=$(case a in a) cat .*;; esac; grep 'a .*b' f)\necho $x", ".*"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ScanBash(tc.cmd)
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("ScanBash(%q) denied on %q — it prints no credential", tc.cmd, got[0].Path)
				}
				return
			}
			if len(got) == 0 {
				t.Fatalf("ScanBash(%q) allowed — this is the leak", tc.cmd)
			}
			if !anyNames(got, tc.want) {
				t.Errorf("ScanBash(%q) = %+v, want a finding naming %q", tc.cmd, got, tc.want)
			}
		})
	}
}

func anyNames(fs []Finding, want string) bool {
	for _, f := range fs {
		if f.Path == want || f.Path == "$x" {
			return true
		}
	}
	return false
}

// GNU sed runs a script's `e` command through the shell; a glob there is held
// by its stem, which is why the exemption is `.*` at the head and no wider.
func TestScanBash_SedExecutesAGlobByItsStem(t *testing.T) {
	cmd := "x=$(sed '1e cat .e* ' f)\necho $x"
	if len(ScanBash(cmd)) == 0 {
		t.Fatalf("ScanBash(%q) allowed — this is the leak", cmd)
	}
}

// A case pattern's `)` closes a substitution early, and the quoting after it
// can still balance. The lexer strips a word's outer quotes before ScanBash
// sees it, so this shape is held at secretInside, which must be sound on any
// word it is handed.
func TestSecretInside_CaseVoidsTheExemption(t *testing.T) {
	word := `"$(case a in a) cat .* ;; esac)"`
	if got := secretInside(word); got != ".*" {
		t.Fatalf("secretInside(%q) = %q, want .*", word, got)
	}
}
