package buildinfo

// ponytail: no build/release pipeline exists yet to inject this via -ldflags;
// upgrade once one does (git describe / release tag → -X buildinfo.Version=...).
var Version = "dev"
