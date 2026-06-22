//go:build linux && !cgo

package main

import "github.com/g-lok/chirashi/cmd"

func main() {
	cmd.Execute()
}
