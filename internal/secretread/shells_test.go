// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package secretread

import (
	"strings"
	"testing"
	"time"
)

// A script handed to a local shell gets the verdict it would get typed directly:
// a read is refused however it is quoted, and what is safe typed stays safe.
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
		`ssh host 'cat /opt/deploy/.env'`,
		`ssh host "cat /opt/deploy/.env"`,
		`ssh -i key root@host 'sudo cat /opt/deploy/.env'`,
		`ssh -p 22 -o BatchMode=yes host 'grep TOKEN /opt/deploy/.env'`,
		`ssh -p22 -tt host 'cd /opt/deploy && cat .env'`,
		`timeout 5 ssh host 'cat .env'`,
		`ssh host <<'EOF'` + "\ncat .env\nEOF",
		`ssh host 'bash -s' <<'EOF'` + "\ncat .env\nEOF",
		`ssh host bash -c "'cat .env'"`,
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
		`ssh host 'wc -c /opt/deploy/.env'`,
		`ssh -i ~/.ssh/id_ed25519 host uptime`,
		`ssh -fN -i ~/.ssh/obsdebug_ed25519 -L 9428:127.0.0.1:9428 obsdebug@159.203.55.241`,
		`cat <<'EOF'` + "\ncat .env\nEOF",
		`echo bash`,
		`grep -n bash README.txt`,
	}
	for _, c := range silent {
		if found := ScanBash(c); len(found) != 0 {
			t.Errorf("refused what prints no credential: %s: %+v", c, found)
		}
	}
}

// A script nested past the bound is still searched for names, and depth costs
// nothing near the hook's timeout.
func TestDepthNeitherHidesAReadNorOutrunsTheHook(t *testing.T) {
	nested := func(core string, depth int) string {
		for range depth {
			core = `bash -c "` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `\$`, "`", "\\`").Replace(core) + `"`
		}
		return core
	}
	for _, depth := range []int{3, maxShells, maxShells + 4} {
		if len(ScanBash(nested("cat .env", depth))) == 0 {
			t.Errorf("a read %d shells deep was not refused", depth)
		}
	}
	start := time.Now()
	if found := ScanBash(nested("echo hi", 12)); len(found) != 0 {
		t.Errorf("refused a harmless nesting: %+v", found)
	}
	if len(ScanBash(strings.Repeat("eval ", 20000)+"cat .env")) == 0 {
		t.Error("a read behind 20000 evals was not refused")
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("took %v against a hook bound of 8s", took)
	}
}
