package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/ruanmonteiro19/observa-lite/internal/agent"
	"github.com/ruanmonteiro19/observa-lite/internal/discovery"
	"github.com/ruanmonteiro19/observa-lite/internal/instrumentation"
	"log"
	"os"
)

func main() {
	args := os.Args[1:]
	var e error
	if len(args) > 0 {
		switch args[0] {
		case "agent":
			args = args[1:]
		case "once":
			d, err := discovery.Detect()
			if err != nil {
				log.Fatal(err)
			}
			json.NewEncoder(os.Stdout).Encode(d)
			return
		case "instrumentation":
			f := flag.NewFlagSet("instrumentation", flag.ExitOnError)
			dir := f.String("directory", "", "export Node module")
			f.Parse(args[1:])
			path, err := instrumentation.Export(*dir)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(path)
			return
		}
	}
	e = agent.Run(args)
	if e != nil {
		log.Fatal(e)
	}
}
