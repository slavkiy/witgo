//go:build windows && amd64

package bridgebin

import _ "embed"

//go:embed bin/windows_amd64.exe.gz
var compressedBridge []byte
