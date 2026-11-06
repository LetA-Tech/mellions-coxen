// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package secretread

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// unreadRoutes print under bash and are not read: what this scan knows it misses.
var unreadRoutes = map[string]bool{
	`bash <(echo 'cat .env')`: true,
}

// Bash, not this package's reading of it, says which rows print: a row that prints
// a fixture credential under real bash and draws no finding is a miss.
func TestNoRowPrintsACredentialUnderBashWithoutARefusal(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("no bash here")
	}
	data, err := os.ReadFile("testdata/bash-rows.txt")
	if err != nil {
		t.Fatal(err)
	}
	const fixture = "FIXTURE-CREDENTIAL-VALUE"
	dir, bin := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(fixture+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stubs := map[string]string{
		"sudo": "#!/bin/bash\nwhile [[ $1 == -* ]]; do case $1 in -i|-n) shift;; *) shift 2;; esac; done\nexec \"$@\"\n",
		"psql": "#!/bin/bash\nexit 0\n",
	}
	for name, body := range stubs {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	printed := 0
	for _, row := range strings.Split(string(data), "\n====\n") {
		row = strings.TrimRight(row, "\n")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, bash, "-c", row)
		cmd.Dir = dir
		cmd.Env = []string{"PATH=" + bin + ":/usr/bin:/bin", "HOME=" + dir, "SHELL=" + bash}
		cmd.WaitDelay = time.Second
		out, _ := cmd.CombinedOutput()
		cancel()
		if !strings.Contains(string(out), fixture) {
			continue
		}
		printed++
		if len(ScanBash(row)) == 0 && !unreadRoutes[row] {
			t.Errorf("prints the fixture under bash and draws no finding: %q", row)
		}
	}
	if printed < 80 {
		t.Errorf("only %d rows printed the fixture, so this ran nothing it claims", printed)
	}
	t.Logf("%d rows printed the fixture", printed)
}
