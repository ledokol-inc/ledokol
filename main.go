package main

import (
	"ledokol/load"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(4)
	test := load.NewTest()

	test.Run()
}
