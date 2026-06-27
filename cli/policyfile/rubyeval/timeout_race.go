//go:build race

package rubyeval

import "time"

// defaultEvalTimeout is scaled far up under the race detector. The engine runs
// a ~25 MB CRuby module on wazero; with -race instrumentation the one-time cold
// wasm compile and the guest execution run roughly 30-50x slower, so the
// production 60s bound would false-trip on the first (compile-paying)
// evaluation. Production binaries are never built with -race, so this larger
// bound only affects `go test -race` and never weakens the runtime DoS bound.
var defaultEvalTimeout = 10 * time.Minute
