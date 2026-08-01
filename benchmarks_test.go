package witgo

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/slavkiy/witgo/internal/bridgebin"
)

func BenchmarkCompareContracts(b *testing.B) {
	expected := Contract{
		Imports: []string{
			"examples:contract/host@1.0.0#process-string",
			"examples:contract/host@1.0.0#write-log",
		},
		Exports: []string{
			"examples:contract/plugin-info@1.0.0#metadata",
			"examples:contract/plugin-info@1.0.0#health",
		},
		Signatures: map[string]string{
			"examples:contract/host@1.0.0#process-string":  "(string)->(string)",
			"examples:contract/host@1.0.0#write-log":       "(string)->()",
			"examples:contract/plugin-info@1.0.0#metadata": "()->(record{name:string,version:string,author:string,description:string})",
			"examples:contract/plugin-info@1.0.0#health":   "()->(result<string,string>)",
		},
	}
	actual := Contract{
		Imports: append([]string(nil), expected.Imports...),
		Exports: append([]string(nil), expected.Exports...),
		Signatures: map[string]string{
			"examples:contract/host@1.0.0#process-string":  "(string)->(string)",
			"examples:contract/host@1.0.0#write-log":       "(string)->()",
			"examples:contract/plugin-info@1.0.0#metadata": "()->(record{name:string,version:string,author:string,description:string})",
			"examples:contract/plugin-info@1.0.0#health":   "()->(result<string,string>)",
		},
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		report, err := CompareContracts(expected, actual)
		if err != nil || !report.Compatible {
			b.Fatalf("CompareContracts = %#v, %v", report, err)
		}
	}
}

func BenchmarkMapJSONRoundTrip(b *testing.B) {
	value := NewMap[string, Option[uint32]](4).
		Put("alpha", Some(uint32(1))).
		Put("beta", Some(uint32(2))).
		Put("gamma", None[uint32]())
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		data, err := json.Marshal(value)
		if err != nil {
			b.Fatal(err)
		}
		var decoded Map[string, Option[uint32]]
		if err := json.Unmarshal(data, &decoded); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTupleValue(b *testing.B) {
	tuple := NewTuple("worker", map[string]any{"some": uint32(42)}, true, float64(1))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		value, present, err := TupleValue[Option[uint32]](tuple, 1)
		if err != nil || !present || value.Value != 42 {
			b.Fatalf("TupleValue = %#v, %t, %v", value, present, err)
		}
	}
}

func BenchmarkRuntimeCallPassthrough(b *testing.B) {
	if _, err := bridgebin.Library(); errors.Is(err, bridgebin.ErrUnavailable) && os.Getenv("WITGO_COMPONENT_LIBRARY") == "" {
		b.Skip("embedded bridge is not present for this platform")
	}
	runtime, err := LoadRuntimeFromBytesWithImports([]byte(passthroughComponent), RuntimeOptions{}, []HostImport{{
		Interface: "test:plugin/host@1.0.0",
		Function:  "process-string",
		Call: func(args []any) (any, error) {
			return strings.ToUpper(args[0].(string)), nil
		},
	}})
	if err != nil {
		b.Skipf("runtime benchmark requires a current bridge: %v", err)
	}
	defer runtime.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		value, err := runtime.Call("test:plugin/api@1.0.0#run", "benchmark")
		if err != nil || value != "BENCHMARK" {
			b.Fatalf("runtime.Call = %#v, %v", value, err)
		}
	}
}
