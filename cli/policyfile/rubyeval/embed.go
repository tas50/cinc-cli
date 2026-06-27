package rubyeval

import "embed"

// shimSource is the Policyfile DSL shim run inside ruby.wasm. It is tiny, so
// (unlike the wasm blob) embedding it directly is fine.
//
//go:embed shim.rb
var shimSource string

// rubyLib holds the vendored, pure-Ruby libraries the shim requires inside the
// wasm VM — currently semverse (Apache-2.0), which gives byte-identical version
// constraint normalization to chef. These are small pure-Ruby files, so they
// are embedded rather than downloaded.
//
//go:embed rubylib
var rubyLib embed.FS
