//go:build linux && arm64

package bridgebin

import _ "embed"

//go:embed bin/linux_arm64.gz
var compressedBridge []byte
