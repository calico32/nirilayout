//go:build profile

package main

import (
	"os"
	"runtime/pprof"
)

func init() {
	prof, err := os.Create("cpu.prof")
	if err != nil {
		panic(err)
	}
	defer prof.Close()
	err = pprof.StartCPUProfile(prof)
	if err != nil {
		panic(err)
	}
	defer pprof.StopCPUProfile()
}
