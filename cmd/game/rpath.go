//go:build (darwin || linux) && !archive && !musl

package main

// #cgo LDFLAGS: -Wl,-rpath,${SRCDIR}/../../lib
import "C"
