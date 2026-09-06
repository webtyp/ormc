package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"webtyp.com/ormc"
)

func main() {
	flag.Parse()

	o := ormc.New()
	o.SetLog(func(messages ...any) {
		fmt.Fprintln(os.Stderr, messages...)
	})
	if err := o.Run(); err != nil {
		log.Fatalf("ormc: %v", err)
	}
}
