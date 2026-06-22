//go:build linux && !cgo

package main

import "github.com/g-lok/rexconverter/cmd"

func main() {
	cmd.Execute()
}
