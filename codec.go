package witgo

// TypeCodec converts between a canonical WIT wire value and its optional
// public Go representation. Generated built-in codecs use direct functions,
// so component calls do not need reflection or a runtime registry.
type TypeCodec[Wire, Go any] interface {
	Encode(Go) (Wire, error)
	Decode(Wire) (Go, error)
}

// TypeCodecFuncs adapts two typed functions to TypeCodec.
type TypeCodecFuncs[Wire, Go any] struct {
	EncodeFunc func(Go) (Wire, error)
	DecodeFunc func(Wire) (Go, error)
}

func (c TypeCodecFuncs[Wire, Go]) Encode(value Go) (Wire, error) { return c.EncodeFunc(value) }
func (c TypeCodecFuncs[Wire, Go]) Decode(value Wire) (Go, error) { return c.DecodeFunc(value) }
