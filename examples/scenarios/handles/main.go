package main

import (
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	"github.com/slavkiy/witgo"
)

func main() {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("cannot resolve handles example directory")
	}
	pluginFile := filepath.Join(filepath.Dir(sourceFile), "component.wasm")

	runtimeValue, err := witgo.LoadRuntime(pluginFile)
	if err != nil {
		log.Fatal(err)
	}
	defer runtimeValue.Close()

	rawHandle, err := runtimeValue.Call("examples:handles/api@1.0.0#make")
	if err != nil {
		log.Fatal(err)
	}
	handle, ok := rawHandle.(witgo.Handle)
	if !ok {
		log.Fatalf("unexpected handle value: %#v", rawHandle)
	}

	value, err := runtimeValue.Call("examples:handles/api@1.0.0#value", handle)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Handle ID:", handle.ID())
	fmt.Println("Handle kind:", handle.Kind())
	fmt.Println("Borrowed value:", value)

	if err := handle.Close(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Closed:", handle.IsClosed())
}
