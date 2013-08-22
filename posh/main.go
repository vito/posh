package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	"launchpad.net/goyaml"

	"github.com/vito/posh"
)

var templateFile = flag.String("template", "", "path to manifest template")
var stubFile = flag.String("stub", "", "path to stub .yml file")

func main() {
	flag.Parse()

	var templateYAML, stubYAML interface{}

	templateFile, err := ioutil.ReadFile(*templateFile)
	if err != nil {
		log.Fatalln("error reading template:", err)
	}

	stubFile, err := ioutil.ReadFile(*stubFile)
	if err != nil {
		log.Fatalln("error reading stub:", err)
	}

	err = goyaml.Unmarshal(templateFile, &templateYAML)
	if err != nil {
		log.Fatalln("error parsing template:", err)
	}

	err = goyaml.Unmarshal(stubFile, &stubYAML)
	if err != nil {
		log.Fatalln("error parsing stub:", err)
	}

	spice := &posh.Spice{Stub: posh.Sanitize(stubYAML)}

	flowed := posh.Sanitize(templateYAML)

	for didFlow := true; didFlow; flowed, didFlow = spice.Flow(flowed) {
	}

	err = posh.CheckResolved(flowed)
	if err != nil {
		log.Fatalln(err)
	}

	rendered, err := goyaml.Marshal(flowed)
	if err != nil {
		log.Fatalln("failed to render manifest:", err)
	}

	fmt.Printf("%s", rendered)
}
