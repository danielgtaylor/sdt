package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/danielgtaylor/sdt"
	"github.com/danielgtaylor/shorthand"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: sdt doc.yaml <params.yaml")
		os.Exit(1)
	}

	doc, err := sdt.NewFromFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	args := []string{}
	if len(os.Args) > 2 {
		args = os.Args[2:]
	}
	params, err := shorthand.GetInput(args)
	if err != nil {
		panic(err)
	}
	if params == nil {
		params = map[string]interface{}{}
	}

	// Validate template output format
	if errs := doc.ValidateTemplate(os.Args[1]); len(errs) > 0 {
		fmt.Println("Error while validating template:")
		for _, err := range errs {
			fmt.Println(err)
		}
		os.Exit(1)
	}

	// Validate params from bp.Schema
	if err := doc.ValidateInput("args", params); err != nil {
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
	if err := doc.ValidateOutput(os.Args[1], rendered); err != nil {
		fmt.Println("Error validating rendered output:")
		fmt.Println(err)
		os.Exit(1)
	}

	out, _ := json.MarshalIndent(rendered, "", "  ")
	fmt.Println(string(out))
}
