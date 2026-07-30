package parser

import "testing"

func TestParseFullWITShape(t *testing.T) {
	src := `package local:demo;

use wasi:io/streams.{input-stream};

interface types {
  record XML {
    value: string,
  }

  record pair {
    x: u32,
    y: u32,
  }

  flags perms {
    read,
    write,
  }

  enum color {
    red,
    green,
  }

  variant maybe-file {
    none,
    some(file),
  }

  resource file {
    constructor(path: string);
    open: static async func(path: string) -> result<file, string>;
    read: func(self: borrow<file>, n: u32) -> list<u8>;
  }

  type pair-list = list<pair>;
  type row = tuple<u32, string>;
  type dict = map<string, option<XML>>;
  type open-result = result<_, string>;
}

world app {
  use types.{pair, file as host-file};
  import types;
  import wasi:cli/run;
  import run: func() -> string;
  export api: interface {
    ping: func() -> string;
  }
  include wasi:cli/imports with { run as local-run };
}`

	file, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if file.Package == nil || file.Package.Name != "demo" {
		t.Fatalf("unexpected package: %#v", file.Package)
	}
	if len(file.Uses) != 1 {
		t.Fatalf("unexpected top-level use count: %d", len(file.Uses))
	}
	if len(file.Decls) != 2 {
		t.Fatalf("unexpected decl count: %d", len(file.Decls))
	}
}
