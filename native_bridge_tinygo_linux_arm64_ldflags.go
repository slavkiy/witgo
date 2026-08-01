//go:build tinygo && cgo && linux && arm64 && !android

package witgo

/*
#cgo LDFLAGS: -L/usr/lib/aarch64-linux-gnu
*/
import "C"
