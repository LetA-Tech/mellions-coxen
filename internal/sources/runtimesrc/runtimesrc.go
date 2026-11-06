// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package runtimesrc reads the running system: what each service logged at
// ERROR or FATAL over the last day, and every container on a configured host
// that is not plainly up. Trackers and CI cannot show a stopped service, and a
// source's errors here are what tells a reader this picture is missing rather
// than quiet.
package runtimesrc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/signal"
)

// Name is the source's name in configuration and in Signal.Source.
const Name = "runtime"

const (
	window = "24h"
	// notProduction keeps production's messages, and the user data they can
	// carry, out of a survey that reaches every session's transcript. It excludes
	// rather than selects because a service's last line can carry no environment.
	notProduction = `-"deployment.environment":=production`
	errorLines    = `_time:` + window + ` severity:in("ERROR","FATAL") ` + notProduction
	// control is what separates a quiet estate from a blind query: a dotted field
	// name unquoted, or spelled with an underscore, matches nothing and raises
	// nothing. Its short window also sees logs that stopped arriving today.
	control = `_time:1h severity:* "service.name":* body:* ` + notProduction + ` | stats count() n`
	totals  = errorLines + ` | stats by ("service.name") count() n, count_uniq(body) m, max(_time) last, count() if (severity:FATAL) fatal`
	// messages selects by name, never by count: the loudest message is not the
	// one that matters, and a count is not this source's to rank by.
	messages = errorLines + ` | stats by ("service.name", body) count() n` +
		` | sort by (body) partition by ("service.name") limit ` + messagesCollected

	// messagesCollected bounds what one service's signal carries, and
	// messagesPrinted its one line; both say how many more there are.
	messagesCollected = "50"
	messagesPrinted   = 5
	messageRunes      = 120
	responseBytes     = 32 << 20
	forwardWait       = 15 * time.Second

	// inspect fails when docker fails or lists nothing, because the substitution
	// then leaves it no argument: a host this source cannot read never looks
	// like a host with nothing wrong. Its format names fields, so a container's
	// environment stays on the host.
	inspect = `docker inspect --format '{{.Name}}|{{.State.Status}}|` +
		`{{if .State.Health}}{{.State.Health.Status}}{{end}}|{{.RestartCount}}|{{.State.StartedAt}}' $(docker ps -aq)`
)

// Options says where the running system is read from.
type Options struct {
	// LogsURL is VictoriaLogs' base URL. With LogsVia it is the address as seen
	// from that host.
	LogsURL string `json:"logs_url"`
	// LogsVia is the ssh arguments reaching a host that can see LogsURL, the
	// destination last. A forward private to the collection is opened through
	// it and closed before Collect returns; empty reads LogsURL directly.
	LogsVia []string `json:"logs_via,omitempty"`
	// ContainerHosts are ssh destinations whose containers are read.
	ContainerHosts []string `json:"container_hosts,omitempty"`
	// SSH is the ssh program; empty means "ssh" on PATH.
	SSH string `json:"-"`
}

// Source reads what Options names.
type Source struct{ o Options }

// New returns a source over o.
func New(o Options) *Source {
	if o.SSH == "" {
		o.SSH = "ssh"
	}
	return &Source{o: o}
}

// Name implements signal.Source.
func (s *Source) Name() string { return Name }

// Collect ignores the scope: a running service is not one of the repositories
// a run was narrowed to, and a narrowed run is still a run that chooses work.
// What one half read is kept when the other half failed.
func (s *Source) Collect(ctx context.Context, _ signal.Scope) ([]signal.Signal, error) {
	var out []signal.Signal
	var failed []string
	if s.o.LogsURL != "" {
		sigs, err := s.logs(ctx)
		out = append(out, sigs...)
		if err != nil {
			failed = append(failed, "logs: "+err.Error())
		}
	}
	for _, host := range s.o.ContainerHosts {
		sigs, err := s.containers(ctx, host)
		out = append(out, sigs...)
		if err != nil {
			failed = append(failed, "containers on "+host+": "+err.Error())
		}
	}
	if len(failed) > 0 {
		return out, errors.New(strings.Join(failed, "; "))
	}
	return out, nil
}

func (s *Source) logs(ctx context.Context) ([]signal.Signal, error) {
	base := s.o.LogsURL
	if len(s.o.LogsVia) > 0 {
		forwarded, stop, err := s.forward(ctx)
		if err != nil {
			return nil, err
		}
		defer stop()
		base = forwarded
	}

	var seen []struct {
		N string `json:"n"`
	}
	if err := query(ctx, base, control, &seen); err != nil {
		return nil, err
	}
	if len(seen) != 1 || seen[0].N == "0" {
		return nil, errors.New("no line in the last hour carries service.name, severity and body: " +
			"the logs stopped arriving or those fields moved, and either way no ERROR count can be read")
	}

	type row struct {
		Service string `json:"service.name"`
		Body    string `json:"body"`
		N       string `json:"n"`
		M       string `json:"m"`
		Last    string `json:"last"`
		Fatal   string `json:"fatal"`
	}
	var perService, perMessage []row
	if err := query(ctx, base, totals, &perService); err != nil {
		return nil, err
	}
	if err := query(ctx, base, messages, &perMessage); err != nil {
		return nil, err
	}

	sort.Slice(perMessage, func(i, j int) bool { return perMessage[i].Body < perMessage[j].Body })
	listed := map[string][]string{}
	for _, r := range perMessage {
		listed[r.Service] = append(listed[r.Service], r.N+"× "+clean(r.Body))
	}
	sort.Slice(perService, func(i, j int) bool { return perService[i].Service < perService[j].Service })
	out := make([]signal.Signal, 0, len(perService))
	for _, r := range perService {
		name := r.Service
		if name == "" {
			name = "(no service.name)"
		}
		all := listed[r.Service]
		printed := all[:min(len(all), messagesPrinted)]
		text := strings.Join(printed, "; ")
		if distinct, err := strconv.Atoi(r.M); err == nil && distinct > len(printed) {
			text += fmt.Sprintf("; and %d more messages", distinct-len(printed))
		}
		fatal := ""
		if r.Fatal != "" && r.Fatal != "0" {
			fatal = ", " + r.Fatal + " FATAL"
		}
		// A day's count reads the same for a service failing now and one fixed an
		// hour ago; the age the survey prints beside it is what tells them apart.
		last, _ := time.Parse(time.RFC3339Nano, r.Last)
		out = append(out, signal.Signal{
			Kind: signal.KindAlert, Source: Name, ID: "logs:" + name, Updated: last,
			Title: fmt.Sprintf("%s ERROR/FATAL lines in %s%s: %s", r.N, window, fatal, text),
			Attrs: map[string]string{"service": name, "lines": r.N, "messages": r.M, "fatal": r.Fatal, "window": window},
			Detail: strings.Join(all, "\n") + "\n" + fmt.Sprintf("LogsQL for the lines themselves: %s %q:=%q",
				errorLines, "service.name", r.Service),
		})
	}
	return out, nil
}

// query decodes VictoriaLogs' stream of JSON objects into dst. An oversized
// response is an error rather than a shorter, honest-looking list.
func query[T any](ctx context.Context, base, q string, dst *[]T) error {
	endpoint := strings.TrimRight(base, "/") + "/select/logsql/query"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(url.Values{"query": {q}}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	limited := &io.LimitedReader{R: resp.Body, N: responseBytes}
	if resp.StatusCode != http.StatusOK {
		head, _ := io.ReadAll(io.LimitReader(limited, 300))
		return fmt.Errorf("VictoriaLogs answered %s: %s", resp.Status, bytes.TrimSpace(head))
	}
	dec := json.NewDecoder(limited)
	for {
		var row T
		err := dec.Decode(&row)
		if errors.Is(err, io.EOF) {
			if limited.N <= 0 {
				return fmt.Errorf("VictoriaLogs' answer ran past %d bytes", responseBytes)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("read VictoriaLogs' answer: %w", err)
		}
		*dst = append(*dst, row)
	}
}

// forward opens the private tunnel and returns its base URL and what closes it.
// The port is its own, so this neither needs nor disturbs the fixed-port tunnel
// another session may hold.
func (s *Source) forward(ctx context.Context) (string, func(), error) {
	target, err := url.Parse(s.o.LogsURL)
	if err != nil || target.Port() == "" {
		return "", nil, errors.New("logs_url names no host:port to forward to")
	}
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	port := free.Addr().(*net.TCPAddr).Port
	free.Close()

	args := []string{"-N", "-o", "BatchMode=yes", "-o", "ExitOnForwardFailure=yes", "-o", "ConnectTimeout=10",
		"-L", fmt.Sprintf("127.0.0.1:%d:%s", port, target.Host)}
	cmd := exec.CommandContext(ctx, s.o.SSH, append(args, s.o.LogsVia...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Start(); err != nil {
		return "", nil, err
	}
	gone := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(gone)
	}()
	stop := func() {
		_ = cmd.Process.Kill()
		<-gone
	}

	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.After(forwardWait)
	for {
		select {
		case <-gone:
			return "", nil, fmt.Errorf("the tunnel to VictoriaLogs did not open: %s", brief(stderr.String()))
		case <-ctx.Done():
			stop()
			return "", nil, ctx.Err()
		case <-deadline:
			stop()
			return "", nil, fmt.Errorf("the tunnel to VictoriaLogs did not answer within %s", forwardWait)
		case <-time.After(200 * time.Millisecond):
		}
		if healthy(ctx, base) {
			return base, stop, nil
		}
	}
}

func healthy(ctx context.Context, base string) bool {
	c, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(c, http.MethodGet, base+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (s *Source) containers(ctx context.Context, host string) ([]signal.Signal, error) {
	cmd := exec.CommandContext(ctx, s.o.SSH, "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", host, inspect)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, brief(stderr.String()))
	}
	var out []signal.Signal
	for line := range strings.Lines(stdout.String()) {
		f := strings.Split(strings.TrimSpace(line), "|")
		if len(f) != 5 {
			return out, fmt.Errorf("unreadable docker inspect line %q", brief(line))
		}
		name, state, health, restarts := strings.TrimPrefix(f[0], "/"), f[1], f[2], f[3]
		var wrong []string
		if state != "running" {
			wrong = append(wrong, state)
		}
		if health == "unhealthy" {
			wrong = append(wrong, health)
		}
		if restarts != "0" {
			wrong = append(wrong, "restarted "+restarts+" time(s) since it was created")
		}
		if len(wrong) == 0 {
			continue
		}
		// RestartCount never falls, so the age of the last start is what tells a
		// crash loop from a restart weeks ago.
		started, _ := time.Parse(time.RFC3339Nano, f[4])
		out = append(out, signal.Signal{
			Kind: signal.KindAlert, Source: Name, ID: "container:" + host + "/" + name, Updated: started,
			Title: "container " + name + " on " + host + ": " + strings.Join(wrong, ", "),
			Attrs: map[string]string{"host": host, "container": name, "state": state, "health": health, "restarts": restarts},
		})
	}
	return out, nil
}

// These cover what a log line is known to carry credentials in: a URL's
// userinfo, a JWT, a bearer token, and anything after a credential's name. A
// value after a name can be quoted, escaped inside wrapped JSON or several words
// long, so the rest of the message goes with it. They are a second line behind
// services not logging credentials, and cannot recognise a bare value nothing
// names.
var (
	urlUserinfo = regexp.MustCompile(`://[^\s@]+@`)
	jwt         = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_.-]+`)
	bearerToken = regexp.MustCompile(`(?i)\bbearer\s+\S{8,}`)
	namedValue  = regexp.MustCompile(`(?i)[a-z0-9_-]*(?:pass(?:word|wd|phrase)?|pwd|secret|token|key|auth(?:orization)?|cookie|session|sig(?:nature)?|credentials?)\\*"?(?:\s*[=:]|\s+is\b).*$`)
)

// clean makes one log message safe and short enough for a line a transcript
// keeps. The credential shapes go first, so the cut cannot leave half of one
// unrecognised.
func clean(body string) string {
	s := strings.Join(strings.Fields(body), " ")
	s = urlUserinfo.ReplaceAllString(s, "://<redacted>@")
	s = jwt.ReplaceAllString(s, "<redacted>")
	s = bearerToken.ReplaceAllString(s, "bearer <redacted>")
	s = namedValue.ReplaceAllString(s, "<redacted>")
	if r := []rune(s); len(r) > messageRunes {
		return string(r[:messageRunes]) + "…"
	}
	return s
}

func brief(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 200 {
		return string(r[:200]) + "…"
	}
	return s
}
