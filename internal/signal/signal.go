// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Package signal is the provider-neutral core of situational awareness.
//
// A Signal is one observed fact about the engineering program: an issue exists,
// a build failed, a claim no longer holds, an assignment is unfinished. Sources
// produce them; the survey assembles them; the model reasons over them.
//
// Nothing here interprets a signal, and nothing here orders one above another.
// That boundary is the whole point of the package: judgment lives in the model,
// collection lives in software, and the moment software starts scoring signals
// it has taken over the decision it was built to inform.
//
// No package under internal/sources may be imported from here. Sources depend
// on the core; the core never depends on a provider.
package signal

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Kind classifies what a signal tells the engineer.
//
// Deliberately small and provider-neutral: a GitHub issue, a GitLab issue and a
// Jira ticket are all KindWorkItem, because the difference between them is a
// detail of where they live rather than of what they mean to an engineer.
type Kind string

const (
	// KindAssignment is work already in flight, held by this engineer.
	KindAssignment Kind = "assignment"
	// KindWorkItem is a tracked unit of work — issue, ticket, card.
	KindWorkItem Kind = "work_item"
	// KindChangeSet is a proposed change under review — PR, MR.
	KindChangeSet Kind = "change_set"
	// KindBuild is the result of an automated check.
	KindBuild Kind = "build"
	// KindAlert is a runtime alarm or measured anomaly.
	KindAlert Kind = "alert"
	// KindCommit is a repository change.
	KindCommit Kind = "commit"
	// KindStalePremise is a recorded claim the current tree contradicts. It is
	// the signal that stops an engineer executing instructions that were true
	// when written and are not true now.
	KindStalePremise Kind = "stale_premise"
	// KindBlocked is work waiting on the owner.
	KindBlocked Kind = "blocked"
	// KindObjective is a standing program objective.
	KindObjective Kind = "objective"
	// KindFollowUp is something found while doing other work.
	KindFollowUp Kind = "follow_up"
)

// Kinds is every kind, in the order a survey renders them. Ordering here is
// presentational grouping only — it says nothing about importance, and must not
// be read as one.
var Kinds = []Kind{
	KindObjective, KindAssignment, KindStalePremise, KindAlert, KindBuild,
	KindBlocked, KindChangeSet, KindWorkItem, KindFollowUp, KindCommit,
}

// Signal is one observed fact.
//
// It carries no priority, score, weight, rank, severity ordering or
// recommendation, and it never will: see TestSignalCarriesNoJudgement, which
// fails the build if such a field appears. Everything a source knows that the
// core does not model goes in Attrs, uninterpreted.
type Signal struct {
	Kind Kind
	// Source is the name of the source that produced this, so a reader can
	// tell where a claim came from and how much it is worth.
	Source string
	// ID is stable within Source and Repo, so the same fact collected twice is
	// one fact. It need not be unique across repositories — Key carries Repo —
	// and a source must not embed the repository in it to make it so.
	ID string
	// Title is one line, as the provider states it.
	Title string
	// URL locates the underlying artifact, empty when there is none.
	URL string
	// Repo is the repository this concerns, empty when it concerns none.
	Repo string
	// Created and Updated are provider timestamps; zero when unknown.
	Created time.Time
	Updated time.Time
	// Labels are provider labels, verbatim and uninterpreted.
	Labels []string
	// Attrs is everything else the source knows. The core never reads it; the
	// model does. This is what keeps provider richness from being flattened
	// into a lowest common denominator.
	Attrs map[string]string
	// Detail is optional supporting text — an excerpt, an error, a measurement.
	Detail string
}

// Key identifies a signal across collections.
//
// Repo is part of the identity because an ID is only unique within the
// repository a source read it from: issue #7, a workflow named CI and a
// citation on #416 exist in most repositories at once. Without Repo, Dedupe
// keeps the first repository's copy and drops every other repository's, so the
// survey reports fewer facts than it collected and reports them as complete —
// the false green a collector exists to prevent.
func (s Signal) Key() string {
	return s.Source + ":" + string(s.Kind) + ":" + s.Repo + ":" + s.ID
}

// Age reports how long since the signal last changed, using Updated when known
// and Created otherwise. Zero when neither is known.
func (s Signal) Age(now time.Time) time.Duration {
	switch {
	case !s.Updated.IsZero():
		return now.Sub(s.Updated)
	case !s.Created.IsZero():
		return now.Sub(s.Created)
	default:
		return 0
	}
}

// Scope bounds what a collection run looks at.
type Scope struct {
	// Repos limits collection to these repositories. Empty means the source
	// decides, which for most sources means everything it is configured for.
	Repos []string
	// Since bounds how far back a time-ordered source reaches. Zero means the
	// source's own default.
	Since time.Time
	// Limit caps how many signals one source may return, so a source with a
	// pathological result set cannot drown every other source. Zero means the
	// source's own default.
	Limit int
}

// Source collects signals of one kind of engineering evidence.
//
// Implementations live under internal/sources and are the only place a provider
// name appears. A source that cannot reach its provider returns an error rather
// than an empty slice: silence and absence must stay distinguishable, because
// "no failing builds" and "could not reach CI" lead to opposite decisions.
type Source interface {
	// Name is stable and appears in Signal.Source.
	Name() string
	// Collect gathers what this source can see within scope.
	Collect(ctx context.Context, scope Scope) ([]Signal, error)
}

// Registry maps source names to constructors so configuration can select
// sources without the core importing any of them. A provider package registers
// itself from an init function; nothing else knows it exists.
type Registry struct {
	byName map[string]Source
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{byName: map[string]Source{}} }

// Register adds a source. A duplicate name is a configuration error rather than
// a silent replacement: two sources answering to one name would make the
// provenance recorded in Signal.Source a lie.
func (r *Registry) Register(s Source) error {
	if s == nil {
		return fmt.Errorf("signal: register nil source")
	}
	name := strings.TrimSpace(s.Name())
	if name == "" {
		return fmt.Errorf("signal: source has no name")
	}
	if _, dup := r.byName[name]; dup {
		return fmt.Errorf("signal: source %q is already registered", name)
	}
	r.byName[name] = s
	return nil
}

// Get returns a registered source.
func (r *Registry) Get(name string) (Source, bool) {
	s, ok := r.byName[name]
	return s, ok
}

// Names lists registered sources in a stable order.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.byName))
	for n := range r.byName {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Dedupe removes signals sharing a Key, keeping the first occurrence.
//
// One source legitimately returns the same fact twice — a repository named
// twice in a scope, an overlapping page of a listing — and counting it twice
// would misrepresent how much is actually happening. Source and Repo are both
// in the Key, so this never merges two sources' readings of one artifact and
// never merges two repositories' identically numbered work.
func Dedupe(in []Signal) []Signal {
	seen := make(map[string]bool, len(in))
	out := make([]Signal, 0, len(in))
	for _, s := range in {
		if seen[s.Key()] {
			continue
		}
		seen[s.Key()] = true
		out = append(out, s)
	}
	return out
}

// GroupByKind buckets signals for rendering. Within a bucket the input order is
// preserved: a source that returns its own natural order keeps it, and the
// grouping adds no order of its own.
func GroupByKind(in []Signal) map[Kind][]Signal {
	out := make(map[Kind][]Signal)
	for _, s := range in {
		out[s.Kind] = append(out[s.Kind], s)
	}
	return out
}
