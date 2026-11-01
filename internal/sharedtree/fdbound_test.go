// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

package sharedtree_test

import (
	"testing"

	"github.com/LetA-Tech/mellions-coxen/internal/sharedtree"
)

// A word bash passes must stay a word, or the option it is the value of takes
// the next one: a number too large for a descriptor, and the 2 of 2>(…), are
// arguments, so git still runs reset here.
func TestAWordBashPassesIsNotReadAsADescriptor(t *testing.T) {
	const checkout = "/home/you/workspace/data-service"
	for _, command := range []string{
		"git --namespace 2147483648>/dev/null reset --hard",
		"git --namespace 2>(true) reset --hard",
	} {
		if sharedtree.Deny(payload("Bash", checkout, command), estate) == "" {
			t.Errorf("Deny(%q) allowed it; bash runs git reset --hard there", command)
		}
	}
}
