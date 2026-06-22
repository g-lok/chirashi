//go:build !linux

package main

import "C"
import "github.com/g-lok/rexconverter/cmd"

//export GoMainEntry
func GoMainEntry() {
	cmd.Execute()
}

func main() {
	// main is empty — Zig entrypoint calls GoMainEntry() via c-archive.
}
