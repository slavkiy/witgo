//go:build darwin && arm64

package bridgebin

import _ "embed"

//go:embed bin/darwin_arm64.dylib.gz
var compressedBridge []byte

//go:embed bin/darwin_arm64.dylib.sha256
var expectedBridgeSHA256 string
