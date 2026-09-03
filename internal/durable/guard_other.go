// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

//go:build !unix

package durable

import "errors"

// Guard refuses rather than running unlocked.
//
// What it protects is an approved action running once and a session's
// responsibility record surviving a concurrent write. Proceeding without the
// lock would lose both silently, which is the failure the lock exists for.
func Guard(string, func() error) error {
	return errors.New("durable: no interprocess lock on this platform, and an unguarded " +
		"claim can release an approved action twice")
}
