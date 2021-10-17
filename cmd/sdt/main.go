package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/danielgtaylor/sdt"
	"gopkg.in/yaml.v3"
)

func mustLoad(filename string, value interface{}) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	if err := yaml.Unmarshal(b, value); err != nil {
		panic(err)
	}
}

func main() {
	doc, err := sdt.NewFromFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	var params map[string]interface{}
	mustLoad(os.Args[2], &params)

	// Validate template output format
	if errs := doc.ValidateTemplate(os.Args[1]); len(errs) > 0 {
		fmt.Println("Error while validating template:")
		for _, err := range errs {
			fmt.Println(err)
		}
		os.Exit(1)
	}

	// Validate params from bp.Schema
	if err := doc.ValidateInput(os.Args[2], params); err != nil {
		fmt.Println("Error while validating input params:")
		fmt.Println(err)
		os.Exit(1)
	}

	// Render the output
	rendered, errs := doc.Render(params)
	if len(errs) > 0 {
		fmt.Println("Error while rendering template:")
		for _, err := range errs {
			fmt.Println(err)
		}
		os.Exit(1)
	}

	// Confirm that the output conforms to the schema now that it's rendered.
	if err := doc.ValidateOutput(os.Args[2], rendered); err != nil {
		fmt.Println("Error validating rendered output:")
		fmt.Println(err)
		os.Exit(1)
	}

	out, _ := json.MarshalIndent(rendered, "", "  ")
	fmt.Println(string(out))
}
