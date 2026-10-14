// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package patternkill

import (
	"strings"
	"testing"
)

func TestFindRefusesASignalChosenByNameOrPattern(t *testing.T) {
	for cmd, want := range map[string]string{
		`pkill -f "go test ./..." -u $(id -u)`:                    `pkill -f go test ./... -u $(id -u)`,
		`pkill -f /home/x/run-required.sh; pkill -f lint-gate.sh`: `pkill -f /home/x/run-required.sh`,
		`pkill -f lint-gate.sh -u 1000`:                           `pkill -f lint-gate.sh -u 1000`,
		`pkill golangci-lint`:                                     `pkill golangci-lint`,
		`pkill -9 -f 'go test'`:                                   `pkill -9 -f go test`,
		`pkill -PIPE -f gate`:                                     `pkill -PIPE -f gate`,
		`pkill -SIGPROF golangci-lint`:                            `pkill -SIGPROF golangci-lint`,
		`pkill -KILL -x postgres`:                                 `pkill -KILL -x postgres`,
		`pkill -u leta`:                                           `pkill -u leta`,
		`pkill -s 0`:                                              `pkill -s 0`,
		`pkill -g 0 -f x`:                                         `pkill -g 0 -f x`,
		`pkill -P 1 -f x`:                                         `pkill -P 1 -f x`,
		`pkill -f -- -P`:                                          `pkill -f -- -P`,
		`sleep 1; timeout 5 pkill -f x || true`:                   `pkill -f x`,
		`sudo pkill -f x`:                                         `pkill -f x`,
		`killall go`:                                              `killall go`,
		`killall -9 -u leta golangci-lint`:                        `killall -9 -u leta golangci-lint`,
		`kill $(pgrep -f run-required.sh)`:                        `kill $(pgrep -f run-required.sh)`,
		`kill -9 "$(pgrep -f gate)" 2>/dev/null`:                  `kill -9 $(pgrep -f gate) 2`,
		"kill `pgrep -f gate`":                                    "kill `pgrep -f gate`",
		`kill $(pidof postgres)`:                                  `kill $(pidof postgres)`,
		`pgrep -f gate | xargs kill`:                              `pgrep -f gate | xargs kill`,
		`pgrep go | xargs -n 1 kill -9`:                           `pgrep go | xargs -n 1 kill -9`,
		`pidof go | xargs -r kill`:                                `pidof go | xargs -r kill`,
	} {
		if got := Find(cmd); got != want {
			t.Errorf("Find(%q) = %q, want %q", cmd, got, want)
		}
	}
}

func TestFindAdmitsASignalChosenByPid(t *testing.T) {
	for _, cmd := range []string{
		`kill 1234`,
		`kill -TERM 1234 5678`,
		`kill $!`,
		`kill "$pid"; kill -0 "$pid" 2>/dev/null || echo gone`,
		`kill %1`,
		`kill -- -4321`,
		`kill $(cat run.pid)`,
		`kill $(pgrep -P $$)`,
		`pkill -P 1234`,
		`pkill -P $$`,
		`pkill --parent=4321 -f x`,
		`pkill --parent 4321`,
		`pkill -TERM -P "$runner"`,
		`pkill -fP 1234 go`,
		`pkill -s 4321`,
		`pkill -g 4321 -f x`,
		`pkill -F run.pid`,
		`pkill --pidfile run.pid`,
		`pgrep -af run-required.sh`,
		`pgrep -f gate | wc -l`,
		`pgrep -P 1234 | xargs kill`,
		`killall -l`,
		`echo pkill -f x`,
		`grep -n 'pkill -f' skills/x.md`,
		`rg "killall go"`,
		`echo kill $(pgrep -f x)`,
	} {
		if got := Find(cmd); got != "" {
			t.Errorf("Find(%q) = %q, want admitted", cmd, got)
		}
	}
}

func TestReasonNamesTheCommandAndThePidAlternative(t *testing.T) {
	r := Reason("pkill -f gate")
	for _, want := range []string{"`pkill -f gate`", "same user", "$!", "kill -0", "pkill -P <pid>"} {
		if !strings.Contains(r, want) {
			t.Errorf("Reason lacks %q:\n%s", want, r)
		}
	}
}
