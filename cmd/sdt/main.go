package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/quick"
	"github.com/alecthomas/chroma/styles"
	"github.com/danielgtaylor/sdt"
	"github.com/danielgtaylor/shorthand"
	"github.com/mattn/go-colorable"
	"github.com/spf13/cobra"
)

func init() {
	// Simple 256-color theme for JSON/YAML output in a terminal.
	styles.Register(chroma.MustNewStyle("cli-dark", chroma.StyleEntries{
		// Used for JSON/YAML/Readable
		chroma.Comment:      "#9e9e9e",
		chroma.Keyword:      "#ff5f87",
		chroma.Punctuation:  "#9e9e9e",
		chroma.NameTag:      "#5fafd7",
		chroma.Number:       "#d78700",
		chroma.String:       "#afd787",
		chroma.StringSymbol: "italic #D6FFB7",
		chroma.Date:         "#af87af",
		chroma.NumberHex:    "#ffd7d7",
	}))
}

var renderExample = `sdt render doc.yaml <params.yaml
sdt render doc.yaml name: Alice, param2: 123
sdt render doc.yaml <params.yaml name: override`

// highlight a block of data with the given lexer.
func highlight(lexer string, data []byte) ([]byte, error) {
	sb := &strings.Builder{}
	if err := quick.Highlight(sb, string(data), lexer, "terminal256", "cli-dark"); err != nil {
		return nil, err
	}
	return []byte(sb.String()), nil
}

func mustLoad(filename string) *sdt.Document {
	doc, err := sdt.NewFromFile(filename)
	if err != nil {
		fmt.Printf("❌ Unable to load %s\n%v", filename, err)
		os.Exit(1)
	}

	// Validate template output format
	if errs := doc.ValidateTemplate(); len(errs) > 0 {
		fmt.Println("❌ Error while validating template:")
		for _, err := range errs {
			fmt.Println(err)
		}
		os.Exit(1)
	}

	return doc
}

func main() {
	root := cobra.Command{
		Long:    "Structured Data Templates",
		Example: "sdt validate doc.yaml\nsdt render doc.yaml <params.yaml some: value, other: 123",
	}

	validate := &cobra.Command{
		Use:   "validate FILENAME",
		Short: "Validate a structured data template",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mustLoad(args[0])
			fmt.Println("✅ Document is valid!")
		},
	}

	render := &cobra.Command{
		Use:     "render FILENAME [<params.yaml] [key: value]...",
		Short:   "Render a structured data template with the given params",
		Example: renderExample,
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			doc := mustLoad(args[0])

			params, err := shorthand.GetInput(args[1:])
			if err != nil {
				fmt.Printf("❌ Error getting input\n%v", err)
				os.Exit(1)
			}
			if params == nil {
				params = map[string]interface{}{}
			}

			// Validate params from template input schema
			if err := doc.ValidateInput(params); err != nil {
				fmt.Println("❌ Error while validating input params:")
				fmt.Println(err)
				os.Exit(1)
			}

			// Render the output
			rendered, errs := doc.Render(params)
			if len(errs) > 0 {
				fmt.Println("❌ Error while rendering template:")
				for _, err := range errs {
					fmt.Println(err)
				}
				os.Exit(1)
			}

			// Confirm that the output conforms to the schema now that it's rendered.
			if err := doc.ValidateOutput(rendered); err != nil {
				fmt.Println("❌ Error validating rendered output:")
				fmt.Println(err)
				os.Exit(1)
			}

			out, _ := json.MarshalIndent(rendered, "", "  ")

			// Only output color if the output isn't redirected and NO_COLOR has not
			// been set by the user in their environment.
			var stdout io.Writer = os.Stdout
			if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
				if os.Getenv("NO_COLOR") == "" {
					out, _ = highlight("json", out)

					// Support colored output across operating systems.
					stdout = colorable.NewColorableStdout()
				}
			}
			fmt.Fprintln(stdout, string(out))
		},
	}

	root.AddCommand(validate)
	root.AddCommand(render)

	root.Execute()
}
