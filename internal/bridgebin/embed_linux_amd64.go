//go:build linux && amd64

package bridgebin

import _ "embed"

//go:embed bin/linux_amd64.gz
var compressedBridge []byte
