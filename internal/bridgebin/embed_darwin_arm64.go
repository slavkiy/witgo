//go:build darwin && arm64

package bridgebin

import _ "embed"

//go:embed bin/darwin_arm64.gz
var compressedBridge []byte
