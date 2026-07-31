//go:build windows && arm64

package bridgebin

import _ "embed"

//go:embed bin/windows_arm64.exe.gz
var compressedBridge []byte
