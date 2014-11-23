config
======


[![Build status](https://ci.appveyor.com/api/projects/status/arvt5gn2qrtgmwgl?svg=true)](https://ci.appveyor.com/project/metakeule/config)

cross plattform configuration tool.

Not ready for consumption yet.


Example
-------

```go
package main

import (
    "fmt"

    "github.com/metakeule/config"
)

var (
    cfg = config.MustNew("example", "0.0.1")
    first  = cfg.NewBool("first", "first option")
    second = cfg.NewString("second", "second arg")

    subcmd = cfg.MustSub("subcmd")
    subcmdExec = project.NewString("exec", "which subcommand to exec")
)

func main() {
    cfg.Run("example is an example app for config", nil)

    if first.Get() {
        fmt.Println("first is true")
    } else {
        fmt.Println("first is false")
    }

    fmt.Printf("first locations: %#v\n", cfg.Locations("extra"))

    fmt.Printf("second is: %#v\n", second.Get())
    fmt.Printf("second locations: %#v\n", cfg.Locations("second"))

    switch cfg.CurrentSub() {
    case nil:
        fmt.Println("no subcommand")
    case project:
        fmt.Println("subcmd subcommand")
        fmt.Println("subcmd exec is: ", subcmdExec.Get())
        fmt.Printf("subcmd exec locations: %#v\n", subcmd.Locations("exec"))
    default:
        panic("must not happen")
    }
}

```