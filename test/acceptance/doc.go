// Package acceptance holds end-to-end acceptance tests that run the cinc
// binary against a real chef-zero server.
//
// These tests are gated behind the "acceptance" build tag and require Ruby
// and the chef-zero gem. Run them with:
//
//	go test -tags acceptance ./test/...
package acceptance
