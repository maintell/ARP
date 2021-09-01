package main

import (
	"runtime"
)

var (
	version  = "1.1.0"
	build    = "Custom"
	codename = "ARP, Application Reverse Proxy."
	intro    = "a reverse proxy with redis support."
)

// Version returns ARP's version as a string, in the form of "x.y.z" where x, y and z are numbers.
// ".z" part may be omitted in regular releases.
func Version() string {
	return version
}

// VersionStatement returns a list of strings representing the full version info.
func VersionStatement() []string {
	return []string{
		Concat("ARP ", Version(), " (", codename, ") ", build, " (", runtime.Version(), " ", runtime.GOOS, "/", runtime.GOARCH, ")"),
		intro,
	}
}
