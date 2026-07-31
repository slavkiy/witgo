//go:build linux && amd64

package bridgebin

import _ "embed"

//go:embed bin/linux_amd64.so.gz
var compressedBridge []byte

//go:embed bin/linux_amd64.so.sha256
var expectedBridgeSHA256 string
