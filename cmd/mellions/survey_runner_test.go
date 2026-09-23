// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// A sources list written before the runner source existed still surveys it.
func TestAConfiguredSourcesListGainsTheRunner(t *testing.T) {
	for name, body := range map[string]string{
		"listed": `{"sources":["programs","assignments"]}`,
		"absent": `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			c, err := loadConfig(p)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(c.Sources, "runner") {
				t.Fatalf("sources %v do not include runner", c.Sources)
			}
			t.Setenv("MELLIONS_HOME", t.TempDir())
			c.Sources = []string{"runner"}
			reg, err := c.build()
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := reg.Get("runner"); !ok {
				t.Fatalf("runner is configured and not built: %v", reg.Names())
			}
		})
	}
}
