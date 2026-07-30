package wasm

import "bytes"

type Kind uint8

const (
	KindUnknown Kind = iota
	KindCoreModule
	KindComponent
)

var (
	magic = []byte{
		0x00, 0x61, 0x73, 0x6D,
	}

	coreHeader = []byte{
		0x00, 0x61, 0x73, 0x6D,
		0x01, 0x00, 0x00, 0x00,
	}

	componentHeader = []byte{
		0x00, 0x61, 0x73, 0x6D,
		0x0D, 0x00, 0x01, 0x00,
	}
)

func IsWasm(data []byte) bool {
	return len(data) >= len(magic) &&
		bytes.Equal(data[:len(magic)], magic)
}

func DetectKind(data []byte) Kind {
	if len(data) < 8 {
		return KindUnknown
	}

	switch {
	case bytes.Equal(data[:8], coreHeader):
		return KindCoreModule

	case bytes.Equal(data[:8], componentHeader):
		return KindComponent

	default:
		return KindUnknown
	}
}
