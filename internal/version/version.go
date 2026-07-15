// Package version holds release metadata injected at build time.
package version

// Version is the semantic release version (overridden via -ldflags on release builds).
var Version = "0.6.0"

// Commit is the git short SHA at build time (overridden via -ldflags on release builds).
var Commit = "dev"
