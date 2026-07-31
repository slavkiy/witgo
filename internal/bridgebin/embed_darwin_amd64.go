//go:build darwin && amd64

package bridgebin

import _ "embed"

//go:embed bin/darwin_amd64.dylib.gz
var compressedBridge []byte

//go:embed bin/darwin_amd64.dylib.sha256
var expectedBridgeSHA256 string
