// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package runtimesrc

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/signal"
	"github.com/LetA-Tech/mellions-coxen/internal/survey"
)

// stubLogs answers the source's three queries the way VictoriaLogs does: one
// JSON object per line, numbers as strings.
func stubLogs(t *testing.T, controlRows, totalRows, messageRows string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			fmt.Fprintln(w, "OK")
			return
		}
		if r.URL.Path != "/select/logsql/query" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		switch q := r.FormValue("query"); q {
		case control:
			fmt.Fprint(w, controlRows)
		case totals:
			fmt.Fprint(w, totalRows)
		case messages:
			fmt.Fprint(w, messageRows)
		default:
			t.Errorf("unexpected query %q", q)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fakeSSH writes an executable standing in for ssh: it prints stdout and exits
// with code, and records its arguments next to itself.
func fakeSSH(t *testing.T, stdout string, code int) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ssh")
	data := filepath.Join(dir, "stdout")
	if err := os.WriteFile(data, []byte(stdout), 0o644); err != nil {
		t.Fatal(err)
	}
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\ncat %q\necho 'Permission denied (publickey).' >&2\nexit %d\n",
		filepath.Join(dir, "args"), data, code)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func printed(t *testing.T, src signal.Source) (survey.Result, string) {
	t.Helper()
	reg := signal.NewRegistry()
	if err := reg.Register(src); err != nil {
		t.Fatal(err)
	}
	runner, err := survey.NewRunner(reg, []string{Name})
	if err != nil {
		t.Fatal(err)
	}
	res := runner.Run(context.Background(), signal.Scope{})
	return res, res.Render(survey.Default())
}

const healthyControl = `{"n":"314315"}` + "\n"

func TestAnUnhealthyServiceReachesThePrintedSurvey(t *testing.T) {
	logs := stubLogs(t, healthyControl,
		`{"service.name":"fininsight","n":"6","m":"1"}`+"\n"+
			`{"service.name":"hipsys-worker-source","n":"282","m":"7","fatal":"1","last":"`+time.Now().Add(-3*time.Hour).UTC().Format(time.RFC3339Nano)+`"}`+"\n"+
			`{"service.name":"finledger","n":"1","m":"1"}`+"\n",
		`{"service.name":"hipsys-worker-source","body":"z loudest message","n":"270"}`+"\n"+
			`{"service.name":"hipsys-worker-source","body":"a readiness probe stopped holding","n":"7"}`+"\n"+
			`{"service.name":"hipsys-worker-source","body":"a source observation round failed","n":"5"}`+"\n"+
			`{"service.name":"fininsight","body":"foundation operation failed","n":"6"}`+"\n"+
			`{"service.name":"finledger","body":"posting refused","n":"1"}`+"\n")
	started := time.Now().Add(-5 * time.Hour).UTC().Format(time.RFC3339Nano)
	ssh := fakeSSH(t, "/hipsys-worker-source-1|running|unhealthy|0|"+started+"\n"+
		"/fxrate|running||0|"+started+"\n"+
		"/connector|running|healthy|0|"+started+"\n"+
		"/outbox-worker|restarting||14|"+started+"\n"+
		"/one-shot|exited||0|"+started+"\n", 0)

	res, out := printed(t, New(Options{LogsURL: logs.URL, ContainerHosts: []string{"root@preprod"}, SSH: ssh}))
	if !res.Complete() {
		t.Fatalf("a source that read everything reported failures: %v", res.Failures)
	}
	for _, want := range []string{
		"## Alerts — runtime and security (6)",
		"**logs:hipsys-worker-source** 3h · 282 ERROR/FATAL lines in 24h, 1 FATAL: 7× a readiness probe stopped holding; 5× a source observation round failed; 270× z loudest message; and 4 more messages",
		"**logs:fininsight** 6 ERROR/FATAL lines in 24h: 6× foundation operation failed",
		"5h · container hipsys-worker-source-1 on root@preprod: unhealthy",
		"container outbox-worker on root@preprod: restarting, restarted 14 time(s)",
		"container one-shot on root@preprod: exited",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the printed survey lacks %q:\n%s", want, out)
		}
	}
	for _, quiet := range []string{"fxrate", "connector"} {
		if strings.Contains(out, quiet) {
			t.Errorf("container %s is up and was listed:\n%s", quiet, out)
		}
	}
	// 6, 1, 282: no ordering by the count, as numbers or as text, gives this one.
	a, b, c := strings.Index(out, "logs:fininsight"), strings.Index(out, "logs:finledger"), strings.Index(out, "logs:hipsys-worker-source")
	if a > b || b > c {
		t.Errorf("services are not in name order, so something else orders them:\n%s", out)
	}
}

func TestAnUnreachableBackendReachesIncomplete(t *testing.T) {
	gone := httptest.NewServer(http.NotFoundHandler())
	url := gone.URL
	gone.Close()

	res, out := printed(t, New(Options{LogsURL: url}))
	if res.Complete() {
		t.Fatalf("an unreachable backend read as complete:\n%s", out)
	}
	if !strings.Contains(out, "## INCOMPLETE") || !strings.Contains(out, "**runtime**: logs:") {
		t.Errorf("the printed survey does not say the source did not answer:\n%s", out)
	}
	if strings.Contains(out, "Every source answered") {
		t.Errorf("the printed survey reports an answer it did not get:\n%s", out)
	}
}

func TestAnUnreachableHostReachesIncompleteAndKeepsTheLogs(t *testing.T) {
	logs := stubLogs(t, healthyControl, `{"service.name":"a","n":"1","m":"1"}`+"\n", `{"service.name":"a","body":"boom","n":"1"}`+"\n")
	ssh := fakeSSH(t, "", 255)
	res, out := printed(t, New(Options{LogsURL: logs.URL, ContainerHosts: []string{"root@preprod"}, SSH: ssh}))
	if res.Complete() {
		t.Fatalf("an unreachable host read as complete:\n%s", out)
	}
	if !strings.Contains(out, "containers on root@preprod") || !strings.Contains(out, "Permission denied") {
		t.Errorf("the failure does not name the host and its cause:\n%s", out)
	}
	if !strings.Contains(out, "**logs:a**") {
		t.Errorf("the half that was read was lost:\n%s", out)
	}
}

func TestAHostWithNothingReadableIsNeverQuiet(t *testing.T) {
	if !strings.Contains(inspect, "$(docker ps -aq)") || strings.Contains(inspect, "xargs") {
		t.Fatalf("inspect must fail with no container to read, not exit 0 with nothing: %s", inspect)
	}
	logs := stubLogs(t, healthyControl, "", "")
	ssh := fakeSSH(t, "not a docker line\n", 0)
	res, out := printed(t, New(Options{LogsURL: logs.URL, ContainerHosts: []string{"root@preprod"}, SSH: ssh}))
	if res.Complete() {
		t.Errorf("output this source cannot parse read as a healthy host:\n%s", out)
	}
}

func TestBlindLogsAreIncompleteNotQuiet(t *testing.T) {
	for name, rows := range map[string]string{
		"the control counts nothing": `{"n":"0"}` + "\n",
		"the control has no row":     "",
	} {
		t.Run(name, func(t *testing.T) {
			logs := stubLogs(t, rows, "", "")
			res, out := printed(t, New(Options{LogsURL: logs.URL}))
			if res.Complete() {
				t.Fatalf("a query that can see nothing read as a quiet estate:\n%s", out)
			}
			if !strings.Contains(out, "service.name, severity and body") {
				t.Errorf("the failure does not say what is blind:\n%s", out)
			}
		})
	}
}

func TestEveryQueryLeavesProductionOut(t *testing.T) {
	for _, q := range []string{control, totals, messages} {
		if !strings.Contains(q, notProduction) {
			t.Errorf("this query reads production's messages: %s", q)
		}
	}
	if strings.Contains(messages, "desc") {
		t.Errorf("messages are selected by how loud they are: %s", messages)
	}
}

func TestAnOversizedAnswerIsAnErrorEvenOnARowBoundary(t *testing.T) {
	row := []byte(`{"n":"12345678"}`)
	if len(row) != 16 || responseBytes%len(row) != 0 {
		t.Fatalf("this row must end exactly on the limit: %d", len(row))
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		chunk := bytes.Repeat(row, 1<<16)
		for range responseBytes/len(chunk) + 1 {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}))
	defer srv.Close()
	var rows []struct {
		N string `json:"n"`
	}
	if err := query(context.Background(), srv.URL, control, &rows); err == nil {
		t.Errorf("a truncated answer read as complete, %d rows", len(rows))
	}
}

func TestAQuietEstateIsCompleteAndEmpty(t *testing.T) {
	logs := stubLogs(t, healthyControl, "", "")
	res, out := printed(t, New(Options{LogsURL: logs.URL}))
	if !res.Complete() || len(res.Signals) != 0 {
		t.Errorf("complete=%v signals=%d:\n%s", res.Complete(), len(res.Signals), out)
	}
}

func TestABackendErrorIsIncomplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "cannot parse query", http.StatusBadRequest)
	}))
	defer srv.Close()
	res, out := printed(t, New(Options{LogsURL: srv.URL}))
	if res.Complete() || !strings.Contains(out, "cannot parse query") {
		t.Errorf("complete=%v:\n%s", res.Complete(), out)
	}
}

func TestCredentialShapesInAMessageDoNotReachTheSurvey(t *testing.T) {
	const secret = "s3cr3tvalue"
	for _, body := range []string{
		"connect postgres://svc_hipsys:" + secret + "@db.example:5432/db failed",
		"auth failed password=" + secret,
		"auth failed PASSWORD: " + secret,
		"dial failed passwd=" + secret + " host=x",
		"bad api_key=" + secret,
		"bad api-key: " + secret,
		"Authorization: Bearer " + secret,
		"bearer " + secret,
		"token=" + secret,
		"PGPASSWORD=" + secret,
		"db_password=" + secret,
		"client_secret: " + secret,
		"access_token=" + secret,
		"x-api-key: " + secret,
		`password="` + secret + ` with spaces"`,
		`{"password":"` + secret + `"}`,
		"https://" + secret + "@github.com/x",
		"postgres://u:pa/" + secret + "@db:5432/x",
		`{\"password\":\"` + secret + `\"}`,
		`failed: {"error":"{\"token\":\"` + secret + `\"}"}`,
		"private_key=" + secret,
		"password: my " + secret,
		"auth=" + secret,
		"Cookie: session=" + secret,
		"GET /x?sig=" + secret,
		"password is " + secret,
		"eyJhbGciOiJIUzI1NiJ9." + secret + ".abc",
		"secret: " + secret,
		"line one\nsecret=" + secret + "\nline three",
		strings.Repeat("x", messageRunes-20) + " password=" + secret,
	} {
		got := clean(body)
		if strings.Contains(got, secret) || strings.Contains(got, secret[:6]) {
			t.Errorf("clean(%q) kept the value: %q", body, got)
		}
	}
	for _, ordinary := range []string{
		"a readiness probe stopped holding",
		"invalid token format",
		"password expired",
		"connect to https://mdb-02:5432/db refused",
		"etl.fx.opening_admission",
	} {
		if got := clean(ordinary); got != ordinary {
			t.Errorf("an ordinary message changed: %q became %q", ordinary, got)
		}
	}
	if got := clean(strings.Repeat("é", messageRunes+5)); len([]rune(got)) != messageRunes+1 {
		t.Errorf("a long message was not cut to %d runes and a mark: %d", messageRunes, len([]rune(got)))
	}
}

// TestMain turns this binary into an ssh that forwards for real when asked, so
// the tunnel's whole life is exercised without a host.
func TestMain(m *testing.M) {
	if target := os.Getenv("RUNTIMESRC_FAKE_SSH_TARGET"); target != "" {
		fakeForward(target)
		return
	}
	os.Exit(m.Run())
}

func fakeForward(target string) {
	var listen string
	for i, a := range os.Args {
		if a == "-L" {
			parts := strings.Split(os.Args[i+1], ":")
			listen = parts[0] + ":" + parts[1]
		}
	}
	to, err := url.Parse(target)
	if err != nil {
		os.Exit(3)
	}
	if err := http.ListenAndServe(listen, httputil.NewSingleHostReverseProxy(to)); err != nil {
		os.Exit(4)
	}
}

func forwardingSSH(t *testing.T, target string) string {
	t.Helper()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ssh")
	script := fmt.Sprintf("#!/bin/sh\necho \"$$ $*\" > %q\nRUNTIMESRC_FAKE_SSH_TARGET=%q exec %q \"$@\"\n", path+".args", target, self)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTheForwardCarriesTheQueriesAndIsGoneAfterCollect(t *testing.T) {
	logs := stubLogs(t, healthyControl, `{"service.name":"a","n":"1","m":"1"}`+"\n", `{"service.name":"a","body":"boom","n":"1"}`+"\n")
	ssh := forwardingSSH(t, logs.URL)

	res, out := printed(t, New(Options{LogsURL: "http://127.0.0.1:9428", LogsVia: []string{"obsdebug@host"}, SSH: ssh}))
	if !res.Complete() || !strings.Contains(out, "**logs:a**") {
		t.Fatalf("complete=%v, failures=%v:\n%s", res.Complete(), res.Failures, out)
	}

	raw, err := os.ReadFile(ssh + ".args")
	if err != nil {
		t.Fatal(err)
	}
	var pid int
	var rest string
	if _, err := fmt.Sscanf(string(raw), "%d", &pid); err != nil {
		t.Fatal(err)
	}
	rest = string(raw)
	for _, want := range []string{"-N", "BatchMode=yes", "ExitOnForwardFailure=yes", ":127.0.0.1:9428", "obsdebug@host"} {
		if !strings.Contains(rest, want) {
			t.Errorf("ssh was not given %q: %s", want, rest)
		}
	}
	listen := strings.Split(strings.SplitAfter(rest, "-L ")[1], ":")
	if c, err := net.Dial("tcp", "127.0.0.1:"+listen[1]); err == nil {
		c.Close()
		t.Errorf("the forward still listens on %s after Collect returned", listen[1])
	}
	if exec.Command("kill", "-0", fmt.Sprint(pid)).Run() == nil {
		t.Errorf("the ssh process %d outlived Collect", pid)
	}
}

func TestATunnelThatCannotOpenIsIncompleteWithItsCause(t *testing.T) {
	ssh := fakeSSH(t, "", 255)
	res, out := printed(t, New(Options{LogsURL: "http://127.0.0.1:9428", LogsVia: []string{"obsdebug@host"}, SSH: ssh}))
	if res.Complete() || !strings.Contains(out, "the tunnel to VictoriaLogs did not open: Permission denied") {
		t.Errorf("complete=%v:\n%s", res.Complete(), out)
	}
}
