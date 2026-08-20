package service

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters for the email+password path (ADR-014 decision #2) —
// argon2id over bcrypt is a deliberate, flagged dependency choice, not
// silently picked. m=64MiB is above OWASP's stated 19MiB argon2id minimum;
// this service has no unusually tight latency budget on login/signup that
// would push toward the lighter end of OWASP's range.
const (
	argon2Time    = 1
	argon2Memory  = 64 * 1024 // KiB
	argon2Threads = 4
	argon2KeyLen  = 32
	argon2SaltLen = 16
)

// dummyPasswordHash is never a real hash of anything — it exists only so
// LoginWithPassword can run verifyPassword against *some* well-formed hash
// for a federated-only account (empty PasswordHash), rather than skipping
// the comparison entirely. Same $argon2id$v=19$m=65536,t=1,p=4$... params
// as hashPassword produces today, so the argon2id cost is identical either
// way — a real wrong-password attempt and a federated-only account take the
// same time, closing the timing side-channel ADR-016 flagged (see
// LoginWithPassword's own doc comment).
const dummyPasswordHash = "$argon2id$v=19$m=65536,t=1,p=4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

// hashPassword returns a self-contained encoded hash (algorithm, params,
// salt, and digest all in one string, PHC-string-style) — never the raw
// password, and never logged by any caller.
func hashPassword(raw string) (string, error) {
	salt := make([]byte, argon2SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("service: generate password salt: %w", err)
	}

	digest := argon2.IDKey([]byte(raw), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argon2Memory, argon2Time, argon2Threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(digest),
	), nil
}

// verifyPassword checks raw against encoded (a string produced by
// hashPassword) in constant time (mirrors otpMatches' discipline). Re-reads
// the parameters embedded in encoded itself, rather than assuming this
// build's current argon2* constants, so a future parameter change can't
// break verification of already-stored hashes.
//
// A package-level var, not a func — lets
// TestLoginWithPassword_AlwaysCallsVerifyPassword swap in a counting
// wrapper around it (ADR-016's timing-side-channel fix needs a test that
// both the federated-only and wrong-password paths through
// LoginWithPassword actually invoke this, without a flaky literal timing
// measurement).
var verifyPassword = func(encoded, raw string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return false
	}

	var memory, time, threads int
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false
	}
	if memory <= 0 || time <= 0 || threads <= 0 || threads > 255 {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	got := argon2.IDKey([]byte(raw), salt, uint32(time), uint32(memory), uint8(threads), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
