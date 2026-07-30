package analysis

import (
	"testing"

	"github.com/slavkiy/witgo/internal/parser/parser"
)

func TestAnalyzeFullWITShape(t *testing.T) {
	src := `package local:demo;

interface types {
  record XML {
    value: string,
  }

  record pair {
    x: u32,
    y: u32,
  }

  variant maybe-file {
    none,
    some(file),
  }

  resource file {
    constructor(path: string);
    open: static func(path: string) -> result<file, string>;
    read: func(self: borrow<file>, n: u32) -> list<u8>;
  }

  type pair-list = list<pair>;
  type row = tuple<u32, string>;
  type dict = map<string, option<XML>>;
  type open-result = result<_, string>;
}

world app {
  use types.{pair, file as host-file};
  import run: func() -> string;
  export api: interface {
    ping: func() -> string;
  }
}`

	file, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if _, err := Analyze(file); err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
}
