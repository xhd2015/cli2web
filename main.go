package main

import (
	"fmt"
	"os"

	"github.com/xhd2015/cli2web/run"
)

func main() {
	err := run.Main(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
