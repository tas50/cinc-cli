//go:build !race

package rubyeval

import "time"

// defaultEvalTimeout bounds a single guest evaluation in normal (non-race)
// builds — the ~5s cold wasm compile on first run plus the actual evaluation.
// It is generous enough for legitimate dynamic Policyfiles while still promptly
// interrupting a runaway guest (e.g. `loop {}`). Overridable per call via
// Options.Timeout.
var defaultEvalTimeout = 60 * time.Second
