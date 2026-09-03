package arch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type homepageDocument struct {
	SchemaVersion int `json:"schemaVersion"`
	Product       struct {
		Name          string            `json:"name"`
		ShortName     string            `json:"shortName"`
		Definition    string            `json:"definition"`
		CanonicalURL  string            `json:"canonicalUrl"`
		RepositoryURL string            `json:"repositoryUrl"`
		SourcePath    string            `json:"sourcePath"`
		Links         map[string]string `json:"links"`
	} `json:"product"`
	Locales map[string]json.RawMessage `json:"locales"`
}

type homepageCard struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type homepageStep struct {
	Label       string `json:"label"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type homepageEvidenceLink struct {
	Label  string `json:"label"`
	Target string `json:"target"`
}

type homepageSupportItem struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Target      string `json:"target"`
}

type homepageComparisonRow struct {
	Label   string `json:"label"`
	Without string `json:"without"`
	With    string `json:"with"`
}

type homepageResource struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	Target      string `json:"target"`
}

type homepageLocale struct {
	Navigation struct {
		Label        string `json:"label"`
		Overview     string `json:"overview"`
		Challenge    string `json:"challenge"`
		Model        string `json:"model"`
		Integrations string `json:"integrations"`
		Evidence     string `json:"evidence"`
		Install      string `json:"install"`
		Community    string `json:"community"`
	} `json:"navigation"`
	Metadata homepageCard `json:"metadata"`
	Hero     struct {
		Eyebrow          string `json:"eyebrow"`
		Title            string `json:"title"`
		Description      string `json:"description"`
		PrimaryCTA       string `json:"primaryCta"`
		SecondaryCTA     string `json:"secondaryCta"`
		IntegrationLabel string `json:"integrationLabel"`
	} `json:"hero"`
	Why struct {
		Title           string         `json:"title"`
		Description     string         `json:"description"`
		IllustrationAlt string         `json:"illustrationAlt"`
		Points          []homepageCard `json:"points"`
	} `json:"why"`
	Model struct {
		Title       string         `json:"title"`
		Description string         `json:"description"`
		Steps       []homepageStep `json:"steps"`
	} `json:"model"`
	Evidence struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Comparison  struct {
			Headers struct {
				Capability string `json:"capability"`
				Without    string `json:"without"`
				With       string `json:"with"`
			} `json:"headers"`
			Rows []homepageComparisonRow `json:"rows"`
		} `json:"comparison"`
		Links []homepageEvidenceLink `json:"links"`
	} `json:"evidence"`
	Support struct {
		Title       string                `json:"title"`
		Description string                `json:"description"`
		Items       []homepageSupportItem `json:"items"`
		LinkLabel   string                `json:"linkLabel"`
	} `json:"support"`
	Install struct {
		Title           string `json:"title"`
		Description     string `json:"description"`
		Command         string `json:"command"`
		Availability    string `json:"availability"`
		Note            string `json:"note"`
		LinkLabel       string `json:"linkLabel"`
		CopyLabel       string `json:"copyLabel"`
		CopiedLabel     string `json:"copiedLabel"`
		CopyFailedLabel string `json:"copyFailedLabel"`
	} `json:"install"`
	Closing struct {
		Title        string             `json:"title"`
		Description  string             `json:"description"`
		PrimaryCTA   string             `json:"primaryCta"`
		SecondaryCTA string             `json:"secondaryCta"`
		Resources    []homepageResource `json:"resources"`
	} `json:"closing"`
}

func TestHomepageContentIsStrictBilingualProjectSource(t *testing.T) {
	root := repoRoot(t)
	contentPath := filepath.Join(root, "website", "homepage.json")
	raw, err := os.ReadFile(contentPath)
	if err != nil {
		t.Fatal(err)
	}

	var document homepageDocument
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		t.Fatalf("decode homepage content: %v", err)
	}
	if document.SchemaVersion != 1 {
		t.Fatalf("schemaVersion = %d, want 1", document.SchemaVersion)
	}
	if document.Product.Name != "Mellions Coxen" {
		t.Fatalf("product name = %q", document.Product.Name)
	}
	if document.Product.ShortName != "Mellions" {
		t.Fatalf("product short name = %q", document.Product.ShortName)
	}
	if document.Product.Definition != "Engineering responsibility and reliability for frontier coding agents." {
		t.Fatalf("product definition = %q", document.Product.Definition)
	}
	if document.Product.CanonicalURL != "https://letatech.ca/mellions-coxen" {
		t.Fatalf("canonical URL = %q", document.Product.CanonicalURL)
	}
	if document.Product.RepositoryURL != "https://github.com/LetA-Tech/mellions-coxen" {
		t.Fatalf("repository URL = %q", document.Product.RepositoryURL)
	}
	if document.Product.SourcePath != "website/homepage.json" {
		t.Fatalf("source path = %q", document.Product.SourcePath)
	}

	wantLinks := map[string]string{
		"repository":    "https://github.com/LetA-Tech/mellions-coxen",
		"documentation": "https://github.com/LetA-Tech/mellions-coxen/blob/main/docs/README.md",
		"installation":  "https://github.com/LetA-Tech/mellions-coxen/blob/main/docs/install.md",
		"integrations":  "https://github.com/LetA-Tech/mellions-coxen/blob/main/docs/integrations.md",
		"benchmarks":    "https://github.com/LetA-Tech/mellions-coxen/blob/main/docs/benchmarks.md",
		"releases":      "https://github.com/LetA-Tech/mellions-coxen/releases",
		"issues":        "https://github.com/LetA-Tech/mellions-coxen/issues",
		"contributing":  "https://github.com/LetA-Tech/mellions-coxen/blob/main/CONTRIBUTING.md",
	}
	if !reflect.DeepEqual(document.Product.Links, wantLinks) {
		t.Fatalf("product links = %#v, want %#v", document.Product.Links, wantLinks)
	}

	if got := sortedKeys(document.Locales); !reflect.DeepEqual(got, []string{"en", "fr"}) {
		t.Fatalf("locales = %v, want [en fr]", got)
	}
	var english, french any
	if err := json.Unmarshal(document.Locales["en"], &english); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(document.Locales["fr"], &french); err != nil {
		t.Fatal(err)
	}
	englishShape := leafShape(english, "")
	frenchShape := leafShape(french, "")
	if !reflect.DeepEqual(englishShape, frenchShape) {
		t.Fatalf("locale shapes differ:\nEN %v\nFR %v", englishShape, frenchShape)
	}
	for localeName, localeRaw := range document.Locales {
		locale := decodeHomepageLocale(t, localeName, localeRaw)
		if len(locale.Why.Points) != 3 {
			t.Errorf("%s why points = %d, want 3", localeName, len(locale.Why.Points))
		}
		if len(locale.Model.Steps) != 4 {
			t.Errorf("%s model steps = %d, want 4", localeName, len(locale.Model.Steps))
		}
		if len(locale.Evidence.Comparison.Rows) != 4 {
			t.Errorf("%s comparison rows = %d, want 4", localeName, len(locale.Evidence.Comparison.Rows))
		}
		if len(locale.Evidence.Links) != 3 {
			t.Errorf("%s evidence links = %d, want 3", localeName, len(locale.Evidence.Links))
		}
		if len(locale.Support.Items) != 3 {
			t.Errorf("%s support items = %d, want 3", localeName, len(locale.Support.Items))
		}
		if len(locale.Closing.Resources) != 4 {
			t.Errorf("%s closing resources = %d, want 4", localeName, len(locale.Closing.Resources))
		}
		wantIdentities := [][3]string{
			{"Claude Code", "claude-code", "integrations"},
			{"Codex", "codex", "integrations"},
			{"GitHub", "github", "repository"},
		}
		for index, want := range wantIdentities {
			if index >= len(locale.Support.Items) {
				break
			}
			item := locale.Support.Items[index]
			if item.Name != want[0] || item.Icon != want[1] || item.Target != want[2] {
				t.Errorf("%s support identity %d = %q/%q/%q, want %q/%q/%q", localeName, index, item.Name, item.Icon, item.Target, want[0], want[1], want[2])
			}
		}
		localeJSON, err := json.Marshal(locale)
		if err != nil {
			t.Fatal(err)
		}
		var localeValue any
		if err := json.Unmarshal(localeJSON, &localeValue); err != nil {
			t.Fatal(err)
		}
		for _, leaf := range leafShape(localeValue, localeName) {
			if strings.HasSuffix(leaf, ":empty") {
				t.Errorf("%s", leaf)
			}
		}
	}

	for _, required := range []string{
		"git clone https://github.com/LetA-Tech/mellions-coxen.git",
		"make install",
		"mellions config init",
		"mellions doctor",
	} {
		if !strings.Contains(string(document.Locales["en"]), required) || !strings.Contains(string(document.Locales["fr"]), required) {
			t.Fatalf("both locales must carry install step %q", required)
		}
	}
}

func decodeHomepageLocale(t *testing.T, name string, raw json.RawMessage) homepageLocale {
	t.Helper()
	var locale homepageLocale
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&locale); err != nil {
		t.Fatalf("decode %s homepage locale: %v", name, err)
	}
	return locale
}

func TestHomepageSchemaAndPublisherAreBoundToTheContentContract(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{"website/homepage.schema.json", ".github/workflows/publish-homepage.yml"} {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if len(bytes.TrimSpace(raw)) == 0 {
			t.Fatalf("%s is empty", rel)
		}
	}

	schemaRaw, err := os.ReadFile(filepath.Join(root, "website", "homepage.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema any
	if err := json.Unmarshal(schemaRaw, &schema); err != nil {
		t.Fatalf("homepage schema is not JSON: %v", err)
	}

	workflowRaw, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "publish-homepage.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(workflowRaw)
	for _, required := range []string{
		"branches: [main]",
		"website/homepage.json",
		"website/homepage.schema.json",
		"LETA_SITE_DISPATCH_TOKEN",
		"mellions-coxen-content-published",
		"client_payload[content_sha]=$CONTENT_SHA",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("publisher does not contain %q", required)
		}
	}
}

func TestPublicProductVocabularyMatchesHomepage(t *testing.T) {
	root := repoRoot(t)
	publicSurfaces := []string{
		"README.md",
		"CONTRIBUTING.md",
		"SECURITY.md",
		"SUPPORT.md",
		"CODE_OF_CONDUCT.md",
		"docs/README.md",
		"docs/architecture.md",
		"docs/benchmarks.md",
		"docs/cli.md",
		"docs/community.md",
		"docs/data-handling.md",
		"docs/install.md",
		"docs/integrations.md",
		"docs/playbook.md",
		"docs/publication.md",
		"docs/repository-metadata.md",
	}
	for _, rel := range publicSurfaces {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "Mellions Engineer") {
			t.Errorf("%s still uses the retired public product name Mellions Engineer", rel)
		}
	}

	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"<h1 align=\"center\">Mellions Coxen</h1>",
		"Engineering responsibility and reliability for frontier coding agents.",
		"A one-click installer or hosted marketplace install is not published today.",
		"https://letatech.ca/mellions-coxen",
	} {
		if !strings.Contains(string(readme), required) {
			t.Errorf("README does not contain canonical public statement %q", required)
		}
	}
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func leafShape(value any, path string) []string {
	var shape []string
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range sortedKeys(typed) {
			shape = append(shape, leafShape(typed[key], path+"/"+key)...)
		}
	case []any:
		shape = append(shape, fmt.Sprintf("%s:length=%d", path, len(typed)))
		for index, item := range typed {
			shape = append(shape, leafShape(item, fmt.Sprintf("%s/%d", path, index))...)
		}
	case string:
		if strings.TrimSpace(typed) == "" {
			shape = append(shape, path+":empty")
		} else {
			shape = append(shape, path+":string")
		}
	default:
		shape = append(shape, fmt.Sprintf("%s:%T", path, typed))
	}
	return shape
}
