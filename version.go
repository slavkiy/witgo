package witgo

import "runtime/debug"

const (
	bridgeProtocolVersion uint32 = 2
	bridgeVersion                = "0.1.0"
)

var bridgeRequiredFeatures = []string{"contract-ping-v1", "typed-signatures-v1"}

func witgoVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "development"
	}
	if info.Main.Path == "github.com/slavkiy/witgo" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	for _, dependency := range info.Deps {
		if dependency.Path == "github.com/slavkiy/witgo" {
			if dependency.Replace != nil {
				dependency = dependency.Replace
			}
			if dependency.Version != "" {
				return dependency.Version
			}
		}
	}
	return "development"
}
