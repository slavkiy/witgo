<p align="center"><img src="assets/art.png" alt="Art" width="300"></p>

## Использование

Сохраните содержимое этого файла в любом месте вашего проекта и запустите его с помощью `go generate`.
``` go
//go:build ignore

package main

import (
    "log"

    "github.com/slavkiy/witgo"
)

func main() {
    err := witgo.Generate(witgo.Config{
        WIT:     "wit",
        Output:  ".",
        Package: "plugin",
    })
    if err != nil {
        log.Fatal(err)
    }
}
```
