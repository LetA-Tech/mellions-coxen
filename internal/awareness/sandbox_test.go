// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package awareness_test

import (
	"strings"
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/awareness"
)

// TestBringingAContainerUpNamesTheCapability is the case: a session starting a
// backend on a shared host is told the toolbox has something for it, once.
func TestBringingAContainerUpNamesTheCapability(t *testing.T) {
	// Every one of these is a line an actual session typed.
	for _, command := range []string{
		`docker run -d --name imp017-authority-pg -p 127.0.0.1:55433:5432 postgres:18.6-alpine`,
		`docker compose up -d`,
		`docker-compose up`,
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
