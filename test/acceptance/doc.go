// Package acceptance holds end-to-end acceptance tests that run the cinc
// binary against a real cinc-zero server.
//
// cinc-zero is a single-binary, in-memory Chef Infra Server
// (https://github.com/tas50/cinc-zero). The harness downloads the pinned
// release once and caches it; set CINC_ZERO_BIN to point at a local build
// instead. The suite skips on platforms cinc-zero does not publish a binary
// for. These tests are gated behind the "acceptance" build tag. Run them with:
//
//	go test -tags acceptance ./test/...
package acceptance
