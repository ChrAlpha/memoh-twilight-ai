// Package sdkdiff checks twilight's error decoding against the providers'
// official Go SDKs. Each test serves one error response and requires twilight
// and the official SDK to read the same fields from it.
//
// It is a separate module so that the official SDKs never enter twilight's
// own dependency graph.
package sdkdiff
