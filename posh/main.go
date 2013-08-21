package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/kylelemons/go-gypsy/yaml"

	"github.com/vito/posh"
)

var templateFile = flag.String("template", "", "path to manifest template")
var stubFile = flag.String("stub", "", "path to stub .yml file")

func main() {
	flag.Parse()

	templateFile, err := yaml.ReadFile(*templateFile)
	if err != nil {
		log.Fatalln(err)
	}

	stubFile, err := yaml.ReadFile(*stubFile)
	if err != nil {
		log.Fatalln(err)
	}

	spice := &posh.Spice{Stub: stubFile.Root}

	flowed := templateFile.Root

	for didFlow := true; didFlow; flowed, didFlow = spice.Flow(flowed) {
	}

	fmt.Println(yaml.Render(flowed))
}
