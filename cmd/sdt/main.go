package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/quick"
	"github.com/alecthomas/chroma/styles"
	"github.com/danielgtaylor/sdt"
	"github.com/danielgtaylor/shorthand"
	"github.com/mattn/go-colorable"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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

	// Colorizes CLI shorthand output when using `-o shorthand`.
	lexers.Register(chroma.MustNewLazyLexer(
		&chroma.Config{
			Name:         "CLI Shorthand",
			Aliases:      []string{"shorthand"},
			NotMultiline: true,
			DotAll:       true,
		},
		func() chroma.Rules {
			return chroma.Rules{
				"whitespace": {
					{Pattern: `\s+`, Type: chroma.Text, Mutator: nil},
				},
				"root": {
					chroma.Include("whitespace"),
					{Pattern: `true|false|null\b`, Type: chroma.KeywordConstant, Mutator: nil},
					{Pattern: `-?[0-9]+\.[0-9]+`, Type: chroma.NumberFloat, Mutator: nil},
					{Pattern: `-?[0-9]+`, Type: chroma.NumberInteger, Mutator: nil},
					{Pattern: `([a-zA-Z0-9_]+)([:{[])`, Type: chroma.ByGroups(chroma.NameTag, chroma.Punctuation), Mutator: nil},
					{Pattern: `"(\\\\|\\"|[^"])*"`, Type: chroma.LiteralStringDouble, Mutator: nil},
					{Pattern: `[{}[\],]`, Type: chroma.Punctuation, Mutator: nil},
					{Pattern: `[^}\],]+`, Type: chroma.LiteralString, Mutator: nil},
				},
			}
		},
	))
}

var useColor bool
var format string

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

func printColor(color int, label string, err error) {
	colorized := label
	if useColor {
		colorized = fmt.Sprintf("\x1b[0;%dm%s:\x1b[0m", color, label)
	}
	fmt.Fprintf(os.Stderr, "%s %v\n", colorized, err)
}

func printResult(result interface{}) {
	var out []byte

	switch format {
	case "yaml":
		out, _ = yaml.Marshal(result)
	case "json":
		out, _ = json.MarshalIndent(result, "", "  ")
	case "shorthand":
		out = []byte(shorthand.Get(result.(map[string]interface{})))
	default:
		panic(fmt.Errorf("unknown format %s", format))
	}

	// Only output color if the output isn't redirected and NO_COLOR has not
	// been set by the user in their environment.
	var stdout io.Writer = os.Stdout
	if useColor {
		out, _ = highlight(format, out)

		// Support colored output across operating systems.
		stdout = colorable.NewColorableStdout()
	}
	fmt.Fprintln(stdout, string(out))
}

func mustLoad(filename string) *sdt.Document {
	doc, err := sdt.NewFromFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Unable to load %s:\n%v", filename, err)
		os.Exit(1)
	}

	// Validate template output format
	warnings, errs := doc.ValidateTemplate()
	for _, warning := range warnings {
		printColor(33, "warning", warning)
	}
	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr, "❌ Error while validating template:")
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "%v\n", err)
		}
		os.Exit(1)
	}

	return doc
}

func main() {
	if fileInfo, _ := os.Stdout.Stat(); (fileInfo.Mode() & os.ModeCharDevice) != 0 {
		if os.Getenv("NO_COLOR") == "" {
			useColor = true
		}
	}

	root := cobra.Command{
		Long:    "Structured Data Templates",
		Example: "sdt validate doc.yaml\nsdt render doc.yaml <params.yaml some: value, other: 123",
	}

	root.PersistentFlags().StringVarP(&format, "output", "o", "json", "Output format [json, yaml, shorthand]")

	validate := &cobra.Command{
		Use:   "validate FILENAME",
		Short: "Validate a structured data template",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			mustLoad(args[0])
			fmt.Println("✅ Document is valid!")
		},
	}

	example := &cobra.Command{
		Use:   "example FILENAME",
		Short: "Generate an example input for a template",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			doc := mustLoad(args[0])
			example, err := doc.Example()
			if err != nil {
				fmt.Fprintf(os.Stderr, "❌ Error generating example\n%v", err)
				os.Exit(1)
			}

			printResult(example)
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
				fmt.Fprintf(os.Stderr, "❌ Error getting input\n%v", err)
				os.Exit(1)
			}
			if params == nil {
				params = map[string]interface{}{}
			}

			// Validate params from template input schema
			if err := doc.ValidateInput(params); err != nil {
				fmt.Fprintln(os.Stderr, "❌ Error while validating input params:")
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			// Render the output
			rendered, errs := doc.Render(params)
			if len(errs) > 0 {
				fmt.Fprintln(os.Stderr, "❌ Error while rendering template:")
				for _, err := range errs {
					fmt.Fprintf(os.Stderr, "%v\n", err)
				}
				os.Exit(1)
			}

			// Confirm that the output conforms to the schema now that it's rendered.
			if err := doc.ValidateOutput(rendered); err != nil {
				fmt.Fprintln(os.Stderr, "❌ Error validating rendered output:")
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}

			printResult(rendered)
		},
	}

	root.AddCommand(example)
	root.AddCommand(validate)
	root.AddCommand(render)

	root.Execute()
}
