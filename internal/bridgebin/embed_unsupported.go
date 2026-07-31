//go:build !(windows || linux || darwin) || !(amd64 || arm64)

package bridgebin

var compressedBridge []byte
