// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package awareness_test

import (
	"strings"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/awareness"
)

// TestBringingAContainerUpNamesTheCapability is the case: a session starting a
// backend on a shared host is told the toolbox has something for it, once.
func TestBringingAContainerUpNamesTheCapability(t *testing.T) {
	// Every one of these is a line a session typed or a harness on this host carries.
	for _, command := range []string{
		`docker run -d --name imp017-authority-pg -p 127.0.0.1:55433:5432 postgres:18.6-alpine`,
		`docker compose up -d`,
		`docker-compose up`,
		`docker compose -f "$COMPOSE_FILE" up -d`,
		`docker-compose -f test/docker-compose.test.yml up -d`,
		`docker compose -p fd1522 --profile db run --rm migrate`,
		`docker compose up; go test ./...`,
		`docker compose up&`,
		`docker compose up|tee log`,
		`(docker compose up)`,
		`bash -c "docker compose up"`,
		`sh -c 'docker-compose up'`,
		`set -e; docker volume create x >/dev/null; docker run --rm alpine true`,
		`podman run --rm alpine true`,
	} {
		notes := awareness.Notes(awareness.Observation{Command: command})
		if !mentions(notes, "mellions-sandbox") {
			t.Errorf("starting a container said nothing about the sandbox: %q", command)
		}
	}
}

// TestOrdinaryDockerReadsSayNothing is the false-positive arm. A note on every
// `docker ps` is a note somebody turns off, and the teardown this exists to ask
// for is itself a `docker rm`.
func TestOrdinaryDockerReadsSayNothing(t *testing.T) {
	for _, command := range []string{
		`docker ps --format '{{.Names}}'`,
		`docker images`,
		`docker system df`,
		`docker rm -f imp017-authority-pg`,
		`docker volume rm imp017-observer-data`,
		`docker compose -f up.yml down -v`,
		`docker compose -f harness.yml logs -f`,
		`docker-compose -f docker-compose.test.yml ps`,
		`docker compose down; echo up`,
		`docker logs imp017-qdrant`,
		`docker exec imp017-authority-pg psql -U postgres -c 'select 1'`,
		`go test ./... -run TestDockerRunner`,
		``,
	} {
		if notes := awareness.Notes(awareness.Observation{Command: command}); mentions(notes, "mellions-sandbox") {
			t.Errorf("an ordinary command was interrupted: %q", command)
		}
	}
}

// TestTheNoteCarriesOneIdentitySoItIsSaidOnce: Said keys on Ident, and a note
// whose identity moved with the command line would be said on every container.
func TestTheNoteCarriesOneIdentity(t *testing.T) {
	first := awareness.Notes(awareness.Observation{Command: "docker run -d a"})
	second := awareness.Notes(awareness.Observation{Command: "docker run -d b"})
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("expected one note each, got %d and %d", len(first), len(second))
	}
	if first[0].Key() != second[0].Key() {
		t.Fatal("two containers produce two identities, so the note repeats all session")
	}
}

func mentions(notes []awareness.Note, what string) bool {
	for _, n := range notes {
		if strings.Contains(n.Do, what) || strings.Contains(n.Because, what) {
			return true
		}
	}
	return false
}

// TestTextThatStartsNothingKeepsTheNote: the note is said once a session, so a
// quoted argument that only mentions a container start would spend it, and the
// session's real start later would hear nothing.
func TestTextThatStartsNothingKeepsTheNote(t *testing.T) {
	for _, command := range []string{
		`gh issue comment 1 --body "use docker run"`,
		`git commit -m "docker run"`,
		`git commit -m 'docker compose up'`,
		`mellions assign open -id x -objective "the note still fires on typed docker run"`,
		`grep -rn "docker run" internal/`,
		`echo "podman run --rm alpine true" > notes.txt`,
		`cat <<'EOF' > run.sh
docker run --rm alpine true
EOF`,
		`git commit -m "$(cat <<'MSG'
fix: the note fired on docker run in a message
MSG
)"`,
		`gh pr create --body "$(cat <<'MSG'
Run docker compose up -d before the suite.
MSG
)"`,
		// A container on another host is that host's, started on its deploy
		// path; the note is about what this host lends.
		`ssh preprod 'docker run --rm alpine true'`,
		`ssh preprod docker compose up -d`,
	} {
		if notes := awareness.Notes(awareness.Observation{Command: command}); mentions(notes, "mellions-sandbox") {
			t.Errorf("text that starts no container here spent the note: %q", command)
		}
	}
}

// TestACommandStringBashRunsStillStartsAContainer: what a line hands a shell, a
// substitution or eval to run is run here, quoted or not.
func TestACommandStringBashRunsStillStartsAContainer(t *testing.T) {
	for _, command := range []string{
		`bash -c 'docker run --rm alpine true'`,
		`sh -c "docker run --rm alpine true"`,
		`bash -lc 'cd x && docker compose up -d'`,
		`sudo sh -c 'docker run --rm alpine true'`,
		`timeout 60 bash -c "docker run --rm alpine true"`,
		`ls | xargs -I{} sh -c 'docker start {}'`,
		`eval "docker run --rm alpine true"`,
		`ID=$(docker run -d alpine sleep 60)`,
		`echo "$(docker run --rm alpine true)"`,
		"ID=`docker create alpine`",
		"echo \"id: `docker create alpine`\"",
		`bash <<'EOF'
docker run --rm alpine true
EOF`,
		`/usr/bin/docker run --rm alpine true`,
		`"docker" run --rm alpine true`,
	} {
		if notes := awareness.Notes(awareness.Observation{Command: command}); !mentions(notes, "mellions-sandbox") {
			t.Errorf("a container start bash runs said nothing: %q", command)
		}
	}
}

// TestManySubstitutionsAreReadInBoundedTime: this runs before every shell
// command, so a line full of substitutions must not cost more than its length.
func TestManySubstitutionsAreReadInBoundedTime(t *testing.T) {
	line := `echo "` + strings.Repeat(`$(printf '%s ' a b) `, 3000) + `"`
	nested := strings.Repeat(`$(echo `, 200) + "x" + strings.Repeat(`)`, 200)
	done := make(chan bool, 1)
	go func() {
		done <- mentions(awareness.Notes(awareness.Observation{Command: line + "; " + nested}), "mellions-sandbox")
	}()
	select {
	case fired := <-done:
		if fired {
			t.Fatal("a line that starts no container spent the note")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("reading a line of substitutions took longer than 5s")
	}
}
