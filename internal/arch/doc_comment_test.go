// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package arch

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestADocCommentBelongsToTheDeclarationBelowIt: a declaration inserted between
// an existing doc comment and the thing that comment documents compiles, passes
// vet, is formatted, and silently moves the documentation onto the newcomer.
//
// It happened here: HandedOffState was added directly under Publish's doc
// comment with no blank line, so godoc printed Publish's prose as the
// constant's and Publish had none at all. Nothing in `make check` — fmt-check,
// vet, test, check-hooks — can see it, and this repository carries no
// golangci-lint or revive, so this test is the whole gate.
//
// The signal is not the Go convention that a doc comment opens with its own
// name; plenty here legitimately do not. It is a doc comment opening with the
// name of a DIFFERENT declaration in the same package, which is what a comment
// left behind by an insertion looks like and is hard to write by accident.
func TestADocCommentBelongsToTheDeclarationBelowIt(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()

	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != root && (strings.HasPrefix(name, ".") || name == "bin" || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		declared := declaredNames(file)
		for _, decl := range file.Decls {
			doc, names := docAndNames(decl)
			if doc == nil || len(names) == 0 {
				continue
			}
			first := firstWord(doc.Text())
			if first == "" || !declared[first] {
				continue
			}
			if slices.Contains(names, first) {
				continue
			}
			rel, _ := filepath.Rel(root, path)
			found = append(found, fmt.Sprintf("%s:%d: doc opens with %s, which is not %s",
				rel, fset.Position(doc.Pos()).Line, first, strings.Join(names, "/")))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	for _, f := range found {
		t.Errorf("a doc comment names a declaration other than the one it is attached to — %s", f)
	}
}

// docAndNames is the doc comment a declaration carries and every name it
// declares. A grouped const or var declares several, and any of them may
// legitimately be what the comment opens with.
func docAndNames(decl ast.Decl) (*ast.CommentGroup, []string) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return d.Doc, []string{d.Name.Name}
	case *ast.GenDecl:
		var names []string
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				names = append(names, s.Name.Name)
			case *ast.ValueSpec:
				for _, n := range s.Names {
					names = append(names, n.Name)
				}
			}
		}
		return d.Doc, names
	}
	return nil, nil
}

// declaredNames is every top-level name the file declares, which is the set a
// stranded doc comment can be pointing at.
func declaredNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, decl := range file.Decls {
		_, declares := docAndNames(decl)
		for _, n := range declares {
			names[n] = true
		}
	}
	return names
}

// firstWord splits on any whitespace, not on a space. The house style here
// opens plenty of doc comments with the name alone on its own line —
// "// Name.\n//\n// prose" — and a space-only split reads that whole first
// line as one word, which no declaration is ever named. That blindness covered
// roughly one doc comment in twenty.
func firstWord(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimFunc(fields[0], func(r rune) bool {
		return !('a' <= r && r <= 'z') && !('A' <= r && r <= 'Z') && !('0' <= r && r <= '9') && r != '_'
	})
}
