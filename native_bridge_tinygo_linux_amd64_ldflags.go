//go:build tinygo && cgo && linux && amd64 && !android

package witgo

/*
#cgo LDFLAGS: -L/usr/lib/x86_64-linux-gnu
*/
import "C"
