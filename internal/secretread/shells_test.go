// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package secretread

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// A script handed to a local shell gets the verdict it would get typed directly:
// a read is refused however it is quoted, and what is safe typed stays safe. What
// ssh runs on the far side is out of this reading.
func TestAScriptHandedToAShellIsReadAsTheCommandLineItIs(t *testing.T) {
	refused := []string{
		`bash -c 'cat .env'`,
		`sh -c "cat /opt/deploy/.env"`,
		`bash -lc 'cat .db_connection'`,
		`bash -c -- 'cat .pgpass'`,
		`sudo -u app bash -c 'cat .env'`,
		`timeout 5 sh -c 'cat .env'`,
		`F=1 sh -c 'cat .env'`,
		`eval 'cat .env'`,
		`eval cat '.env'`,
		`sh <<'EOF'` + "\ncat .env\nEOF",
		`bash -s <<<'cat .env'`,
		`bash -c "bash -c 'cat .env'"`,
		`bash -c 'cat<.env'`,
		`docker exec app sh -c 'cat /app/.env'`,
		`docker compose exec -T app bash -lc 'cat .env'`,
		`kubectl exec pod -- sh -c 'cat .env'`,
		`find . -name x -exec sh -c 'cat .env' \;`,
	}
	for _, c := range refused {
		if len(ScanBash(c)) == 0 {
			t.Errorf("not refused: %s", c)
		}
	}
	silent := []string{
		`bash -c 'echo hi'`,
		`bash -c 'wc -c .env'`,
		`bash -c 'V=$(cat .env); psql "$V"'`,
		`bash script.sh`,
		`sh ./run.sh arg`,
		`eval "$(ssh-agent -s)"`,
		`ssh host uptime`,
		`ssh host 'ls -la /opt/deploy'`,
		`ssh -i ~/.ssh/id_ed25519 host uptime`,
		`ssh -fN -i ~/.ssh/obsdebug_ed25519 -L 9428:127.0.0.1:9428 obsdebug@159.203.55.241`,
		`cat <<'EOF'` + "\ncat .env\nEOF",
		`ssh host 'grep TOKEN /opt/deploy/.env'`,
		`ssh host bash -c 'cat /opt/deploy/.env'`,
		`echo bash`,
		`grep -n bash README.txt`,
	}
	for _, c := range silent {
		if found := ScanBash(c); len(found) != 0 {
			t.Errorf("refused what prints no credential: %s: %+v", c, found)
		}
	}
}

// A shell or eval is read where it is the program, or where a program named as
// running its operands execs it. The same word as data is no script.
func TestAShellIsReadAtCommandPositionNotAtAnyWord(t *testing.T) {
	heredoc := func(head string) string { return head + " <<EOF\nsee .env and cat .env\nEOF" }
	refused := []string{
		`(sh -c 'cat .env')`,
		`(eval 'cat .env')`,
		`( bash -c 'cat .env' )`,
		`if bash -c 'cat .env'; then :; fi`,
		`while sh -c 'cat .env'; do :; done`,
		`case x in a) sh -c 'cat .env';; esac`,
		`f() { sh -c 'cat .env'; }`,
		`builtin eval 'cat .env'`,
		`exec -a name sh -c 'cat .env'`,
		`sudo -iu app bash -c 'cat .env'`,
		`sudo --user app bash -c 'cat .env'`,
		`"$SHELL" -c 'cat .env'`,
		`"$SHELL" <<'EOF'` + "\ncat .env\nEOF",
		`${BASH} -s <<'EOF'` + "\ncat .env\nEOF",
		`$0 <<'EOF'` + "\ncat .env\nEOF",
		`docker exec ssh sh -c 'cat .env'`,
		`sudo -u ssh bash -c 'cat .env'`,
		`docker exec -w /etc/ssh app sh -c 'cat .env'`,
		`docker run --rm app sh -c 'cat .env'`,
		`docker exec -i app sh <<'EOF'` + "\ncat .env\nEOF",
		`xargs -n1 sh -c 'cat .env'`,
		`find . -exec bash -c 'cat .env' \;`,
		`eval -- 'cat .env'`,
		`function f { sh -c 'cat .env'; }`,
		`/usr/bin/env sh -c 'cat .env'`,
		`$0 -c 'cat .env'`,
		`stdbuf -oL sh -c 'cat .env'`,
		`flock /tmp/l bash -c 'cat .env'`,
		`busybox sh -c 'cat .env'`,
		`$(which sh) -c 'cat .env'`,
		`su app -c 'cat .env'`,
		`sudo su - app -c 'cat .env'`,
		`runuser -l app --command 'cat .env'`,
		`watch 'cat .env'`,
		`watch -n 5 -d 'cat .env'`,
		`echo 'cat .env' | tee /dev/null | sh`,
		`eval "$(echo cat .env)"`,
		`sh -c "$(echo cat .env)"`,
		`sh /dev/stdin <<'EOF'` + "\ncat .env\nEOF",
		`sh 0<<'EOF'` + "\ncat .env\nEOF",
		`echo 'cat .env' | sh`,
		`printf 'cat .env' | bash -s`,
		`cat <<'EOF' | bash` + "\ncat .env\nEOF",
	}
	for _, c := range refused {
		if len(ScanBash(c)) == 0 {
			t.Errorf("not refused: %s", c)
		}
	}
	silent := []string{
		heredoc(`gh issue create --title sh --body-file -`),
		heredoc(`git commit --author bash -F -`),
		heredoc(`grep bash`),
		heredoc(`cat`),
		heredoc(`sh 3`),
		heredoc(`eval : 3`),
		`sh 3<<EOF` + "\ncat .env\nEOF",
		`sh 3<<<'cat .env'`,
		`echo bash -c 'cat .env'`,
		`grep -rn sh -c 'cat .env' docs`,
		`gh issue create --title eval --body 'cat .env'`,
		`echo eval 'cat .env'`,
		`eval -- 'wc -c .env'`,
		`sudo -u app echo bash -c 'cat .env'`,
		`docker logs sh`,
		`find . -name sh`,
		`echo 'cat .env' | grep cat`,
		`echo 'cat .env' ; sh`,
		`watch -n 5 'wc -c .env'`,
		`su app -c 'wc -c .env'`,
		`$PY -c 'print(1)'`,
		`$PSQL <<EOF` + "\ncat .env\nEOF",
		`echo 'cat .env' | xargs -n1 sh`,
		`ssh host 'sh -c "cat .env"'`,
	}
	for _, c := range silent {
		if found := ScanBash(c); len(found) != 0 {
			t.Errorf("refused what prints no credential: %s: %+v", c, found)
		}
	}
}

// A variable crosses into a script a shell is given and out of an eval, which runs
// in this shell. What a child shell assigns stays with it.
func TestAVariableCrossesIntoAScriptAndOutOfAnEval(t *testing.T) {
	refused := []string{
		`F=.env; eval 'cat "$F"'`,
		`F=.env; bash -c "cat \"$F\""`,
		`F=.env sh -c 'cat "$F"'`,
		`X=$(cat .env); bash -c "echo $X"`,
		`X=$(cat .env); eval 'echo "$X"'`,
		`eval 'X=$(cat .env)'; echo "$X"`,
		`eval 'F=.env'; cat "$F"`,
		`eval 'eval "F=.env"'; cat "$F"`,
		`X='cat .env'; eval "$X"`,
		`X='cat .env'; bash -c "$X"`,
		`X='cat .env'; bash -c '$X'`,
	}
	for _, c := range refused {
		if len(ScanBash(c)) == 0 {
			t.Errorf("not refused: %s", c)
		}
	}
	silent := []string{
		`F=.env; eval 'wc -c "$F"'`,
		`X=$(cat .env); bash -c "psql $X"`,
		`eval 'X=$(cat .env)'; psql "$X"`,
		`bash -c 'X=$(cat .env)'; echo "$X"`,
		`bash -c 'F=.env; :'; cat "$F"`,
		`X='wc -c .env'; eval "$X"`,
	}
	for _, c := range silent {
		if found := ScanBash(c); len(found) != 0 {
			t.Errorf("refused what prints no credential: %s: %+v", c, found)
		}
	}
}

// A body whose delimiter is unquoted reaches a shell with its escapes resolved,
// and bash runs what is live in it first. A quoted delimiter's body is literal.
func TestAHeredocIsTheTextBashDelivers(t *testing.T) {
	refused := []string{
		"sh <<EOF\nx=\\$(cat .env)\necho \\$x\nEOF",
		"sh <<EOF\necho \\`cat .env\\`\nEOF",
		"cat <<EOF\n$(cat .env)\nEOF",
		"cat <<EOF\n`cat .env`\nEOF",
		"sh <<EOF\necho $(cat .env)\nEOF",
		"sh <<'EOF'\nx=$(cat .env)\necho $x\nEOF",
	}
	for _, c := range refused {
		if len(ScanBash(c)) == 0 {
			t.Errorf("not refused: %q", c)
		}
	}
	silent := []string{
		"cat <<'EOF'\n$(cat .env)\nEOF",
		"cat <<\\EOF\n$(cat .env)\nEOF",
		"cat <<EOF\n\\$(cat .env)\nEOF",
		"cat <<EOF\n\\`cat .env\\`\nEOF",
		"psql <<EOF\n$(cat .env)\nEOF",
		"sh <<EOF\nx=\\$(cat .env)\npsql \\$x\nEOF",
		"cat > note.md <<EOF\nrun $(date) in .env\nEOF",
	}
	for _, c := range silent {
		if found := ScanBash(c); len(found) != 0 {
			t.Errorf("refused what prints no credential: %q: %+v", c, found)
		}
	}
}

// A shell started with no script of its own reads what its parent's stdin holds.
func TestAShellInheritsItsParentsStdin(t *testing.T) {
	const body = " <<'EOF'\ncat .env\nEOF"
	refused := []string{
		`sh -c 'sh'` + body,
		`eval sh` + body,
		`{ sh; }` + body,
		`(sh)` + body,
		`(sh; true)` + body,
		`while true; do sh; done` + body,
		`sh -c 'sh -c "sh"'` + body,
		`echo 'cat .env' | sh -c 'sh'`,
	}
	for _, c := range refused {
		if len(ScanBash(c)) == 0 {
			t.Errorf("not refused: %q", c)
		}
	}
	silent := []string{
		`sh -c 'cat > note'` + " <<'EOF'\nsee .env\nEOF",
		`{ cat; }` + " <<'EOF'\nsee .env\nEOF",
		`while read l; do echo "$l"; done` + " <<'EOF'\nsee .env\nEOF",
	}
	for _, c := range silent {
		if found := ScanBash(c); len(found) != 0 {
			t.Errorf("refused what prints no credential: %q: %+v", c, found)
		}
	}
}

// Depth gets the verdict the text gets at any depth under the bound, and past the
// bound nothing is established, so the command is refused without naming a file.
func TestDepthNeitherHidesAReadNorOutrunsTheHook(t *testing.T) {
	nested := func(core string, depth int) string {
		for range depth {
			core = `bash -c "` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `\$`, "`", "\\`").Replace(core) + `"`
		}
		return core
	}
	heredocs := func(core string, depth int) string {
		for d := range depth {
			delim := fmt.Sprintf("E%d", d)
			core = "sh <<'" + delim + "'\n" + core + "\n" + delim
		}
		return core
	}
	for _, depth := range []int{3, 16, 20} {
		if len(ScanBash(nested("cat .env", depth))) == 0 {
			t.Errorf("a read %d shells deep was not refused", depth)
		}
	}
	for _, depth := range []int{3, 17, 40} {
		for _, core := range []string{"cat .env", `cat .e"nv"`, "cat .e\\nv"} {
			if len(ScanBash(heredocs(core, depth))) == 0 {
				t.Errorf("%s under %d heredocs was not refused", core, depth)
			}
		}
		if found := ScanBash(heredocs("wc -c .env", depth)); len(found) != 0 {
			t.Errorf("wc -c .env under %d heredocs was refused: %+v", depth, found)
		}
	}

	nest := func(depth int) string {
		return strings.Repeat("echo $(", depth) + strings.Repeat(")", depth)
	}
	doubling := func(depth int) string {
		var b strings.Builder
		b.WriteString("A0=:\n")
		for d := 1; d <= depth; d++ {
			fmt.Fprintf(&b, "A%d='eval \"$A%d\"; eval \"$A%d\"'\n", d, d-1, d-1)
		}
		fmt.Fprintf(&b, `eval "$A%d"`, depth)
		return b.String()
	}
	for _, c := range []string{nest(60), "sh <<'EOF'\n" + nest(60) + "\nEOF"} {
		if found := ScanBash(c); len(found) != 0 {
			t.Errorf("a harmless substitution nest was not read: %+v", found)
		}
	}
	if found := ScanBash(doubling(40)); len(found) != 1 || !found[0].Unread {
		t.Errorf("scripts that double through variables must end in a refusal: %+v", found)
	}

	start := time.Now()
	if found := ScanBash(nested("echo hi", 12)); len(found) != 0 {
		t.Errorf("refused a harmless nesting: %+v", found)
	}
	for name, c := range map[string]string{
		"evals":        strings.Repeat("eval ", 20000) + "cat .env",
		"repeats":      strings.Repeat("eval 'eval \"eval x; eval x\"'; ", 40000),
		"operands":     "docker exec a " + strings.Repeat("sh -o ", 40000) + "z",
		"itself":       `X='eval "$X"'; eval "$X"`,
		"substitution": "sh <<'EOF'\n" + nest(240) + "\nEOF",
		"wide":         strings.Repeat("echo $(date) ", 8000),
		"deep":         nest(20000),
		"doubling":     doubling(40),
	} {
		ScanBash(c)
		if took := time.Since(start); took > 6*time.Second {
			t.Errorf("%s took %v against a hook bound of 8s", name, took)
		}
		start = time.Now()
	}
	if len(ScanBash(strings.Repeat("eval ", 20000)+"cat .env")) == 0 {
		t.Error("a read behind 20000 evals was not refused")
	}
	found := ScanBash(strings.Repeat("eval ", 20000) + "wc -c note")
	if len(found) != 1 || !found[0].Unread || !strings.Contains(found[0].Reason(), "cannot read") {
		t.Errorf("a command past the bound established nothing and must say so: %+v", found)
	}
	if time.Since(start) > 6*time.Second {
		t.Errorf("past the bound took %v", time.Since(start))
	}
	if found := ScanBash(strings.Repeat("sh -o ", 80000)); len(found) != 0 {
		t.Errorf("a shell with no script holds no read: %+v", found)
	}
}
