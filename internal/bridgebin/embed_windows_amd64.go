//go:build windows && amd64

package bridgebin

import _ "embed"

//go:embed bin/windows_amd64.dll.gz
var compressedBridge []byte

//go:embed bin/windows_amd64.dll.sha256
var expectedBridgeSHA256 string
