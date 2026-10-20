// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/awareness"
)

// sandboxNoted drives the hook's observation for one command line run from dir
// and reports whether the session would be pointed at the sandbox Skill.
func sandboxNoted(t *testing.T, dir, command string) bool {
	t.Helper()
	root := t.TempDir()
	b, err := json.Marshal(map[string]string{
		"report_root":      filepath.Join(root, "reports"),
		"assignments_root": filepath.Join(root, "assignments"),
	})
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(root, "config.json")
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	was := hookCommand
	hookCommand = command
	t.Cleanup(func() { hookCommand = was })
	for _, n := range awareness.Notes(observe(cfg, dir, "", "claude")) {
		if strings.Contains(n.Do, "mellions:mellions-sandbox") {
			return true
		}
	}
	return false
}

func writeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// The harness two lanes reused: its trap removed the brokers without -v, and
// the note that names the teardown check never fired because the docker run
// was one level down.
const brokerScript = `#!/usr/bin/env bash
set -euo pipefail
trap 'docker rm -f rp-a rp-b >/dev/null' EXIT
docker run -d --name rp-a redpandadata/redpanda:v25.2.1 redpanda start
go test ./... -run Integration
`

// TestAScriptThatStartsAContainerNamesTheSandbox: the container a session
// starts through a local script or make target is the same borrowed resource
// as one it types.
func TestAScriptThatStartsAContainerNamesTheSandbox(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, filepath.Join(dir, "rp-arms.sh"), brokerScript)
	writeScript(t, filepath.Join(dir, "scripts", "pg-harness.sh"),
		"#!/usr/bin/env bash\ncase \"$1\" in\nup) docker compose -f harness.yml up -d ;;\ndown) docker compose down -v ;;\nesac\n")
	writeScript(t, filepath.Join(dir, "Makefile"),
		"## build\nbuild:\n\tgo build ./...\n\ntest-db-up:\n\t@bash scripts/pg-harness.sh up\n")
	writeScript(t, filepath.Join(dir, "sub", "up.sh"), brokerScript)

	if !sandboxNoted(t, dir, `docker run --rm alpine true`) {
		t.Fatal("positive control: a typed docker run is not noted, so nothing below is tested")
	}
	for _, command := range []string{
		`./rp-arms.sh 2>&1 | tee /tmp/arms.log`,
		`bash rp-arms.sh`,
		`set -e; GOFLAGS=-count=1 ./rp-arms.sh`,
		filepath.Join(dir, "rp-arms.sh"),
		`make test-db-up`,
		`make -C ` + dir + ` test-db-up`,
		`cd sub && ./up.sh`,
	} {
		if !sandboxNoted(t, dir, command) {
			t.Errorf("a container started one level down said nothing about the sandbox: %q", command)
		}
	}
}

// TestAScriptThatOnlyReadsOrRemovesSaysNothing is the false-positive arm: the
// teardown this note asks for is itself a script full of docker rm.
func TestAScriptThatOnlyReadsOrRemovesSaysNothing(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, filepath.Join(dir, "teardown.sh"),
		"#!/usr/bin/env bash\ndocker rm -f -v rp-a rp-b\ndocker ps -a\ndocker volume ls -qf dangling=true\n")
	writeScript(t, filepath.Join(dir, "Makefile"), "build:\n\tgo build ./...\n")
	writeScript(t, filepath.Join(dir, "big.sh"), strings.Repeat("#\n", 1<<20)+"docker run alpine\n")
	writeScript(t, filepath.Join(dir, "bin", "tool"), "\x7fELF\x02\x01\x01\x00usage: tool wraps docker run\n")
	for _, command := range []string{
		`./teardown.sh`,
		`bash teardown.sh`,
		`make build`,
		`make`,
		`./missing.sh`,
		`./big.sh`,
		`./bin/tool --help`,
		`go test ./...`,
		`cat rp-arms.sh`,
	} {
		if sandboxNoted(t, dir, command) {
			t.Errorf("a command that starts no container was interrupted: %q", command)
		}
	}
}
