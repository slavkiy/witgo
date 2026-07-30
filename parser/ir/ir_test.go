package ir

import (
	"testing"

	"github.com/slavkiy/witgo/parser/parser"
)

func TestLowerFullWITShape(t *testing.T) {
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

	file, err := parser.Parse(src)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	lowered, err := Lower(file)
	if err != nil {
		t.Fatalf("lower failed: %v", err)
	}

	if lowered.Package == nil || lowered.Package.Name != "demo" {
		t.Fatalf("unexpected package: %#v", lowered.Package)
	}
	if len(lowered.Uses) != 1 {
		t.Fatalf("unexpected top-level use count: %d", len(lowered.Uses))
	}
	if len(lowered.Decls) != 2 {
		t.Fatalf("unexpected decl count: %d", len(lowered.Decls))
	}

	iface, ok := lowered.Decls[0].(*Interface)
	if !ok {
		t.Fatalf("first decl is %T, want *Interface", lowered.Decls[0])
	}
	if iface.Name != "types" {
		t.Fatalf("unexpected interface name: %q", iface.Name)
	}
	if len(iface.Items) != 10 {
		t.Fatalf("unexpected interface item count: %d", len(iface.Items))
	}

	resource, ok := iface.Items[5].(*Resource)
	if !ok {
		t.Fatalf("resource item is %T, want *Resource", iface.Items[5])
	}
	if len(resource.Items) != 3 {
		t.Fatalf("unexpected resource item count: %d", len(resource.Items))
	}
	openFn, ok := resource.Items[1].(*Func)
	if !ok {
		t.Fatalf("resource method is %T, want *Func", resource.Items[1])
	}
	if !openFn.Static || !openFn.Async {
		t.Fatalf("expected static async function, got static=%v async=%v", openFn.Static, openFn.Async)
	}

	world, ok := lowered.Decls[1].(*World)
	if !ok {
		t.Fatalf("second decl is %T, want *World", lowered.Decls[1])
	}
	if len(world.Items) != 6 {
		t.Fatalf("unexpected world item count: %d", len(world.Items))
	}
	if _, ok := world.Items[2].(*Import); !ok {
		t.Fatalf("world import item is %T, want *Import", world.Items[2])
	}
	export, ok := world.Items[4].(*Export)
	if !ok {
		t.Fatalf("world export item is %T, want *Export", world.Items[4])
	}
	if _, ok := export.Extern.(*InlineInterface); !ok {
		t.Fatalf("export extern is %T, want *InlineInterface", export.Extern)
	}
}
