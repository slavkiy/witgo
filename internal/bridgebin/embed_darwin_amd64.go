//go:build darwin && amd64

package bridgebin

import _ "embed"

//go:embed bin/darwin_amd64.gz
var compressedBridge []byte
