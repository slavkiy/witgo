//go:build tinygo && cgo && linux && !android

package witgo

/*
#cgo LDFLAGS: -ldl
#cgo amd64 LDFLAGS: -L/usr/lib/x86_64-linux-gnu
#cgo arm64 LDFLAGS: -L/usr/lib/aarch64-linux-gnu
*/
import "C"
