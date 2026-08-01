//go:build tinygo && cgo && linux && !android

package witgo

/*
#cgo LDFLAGS: -ldl
*/
import "C"
