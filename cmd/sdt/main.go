package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
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
var verbose bool

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

func colorize(color int, value string) string {
	if useColor {
		return fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", color, value)
	}
	return value
}

func printColor(color int, label string, err error) {
	fmt.Fprintf(os.Stderr, "%s %v\n", colorize(color, label+":"), err)
}

func printResult(result interface{}) {
	var out []byte

	switch format {
	case "yaml":
		out, _ = yaml.Marshal(result)
	case "json", "default":
		format = "json"
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

func printWarnings(warnings []sdt.ContextError) {
	for _, warning := range warnings {
		printColor(33, "warning", warning)
	}
}

func exitErr(code int, msg string, err error) {
	if format != "default" {
		printResult([]interface{}{map[string]interface{}{
			"message": err.Error(),
		}})
		os.Exit(code)
	}

	fmt.Fprintf(os.Stderr, "%s %s\n", msg, err.Error())
	os.Exit(code)
}

// colorMarkerRegex is used to colorize source code markers when printing
// out errors in a terminal.
var colorMarkerRegex = regexp.MustCompile(`(?m)^\s+\^+`)

func exit(code int, msg string, warnings []sdt.ContextError, errs []sdt.ContextError) {
	if format != "default" {
		combined := []interface{}{}
		for _, x := range [][]sdt.ContextError{warnings, errs} {
			for _, y := range x {
				combined = append(combined, map[string]interface{}{
					"path":    y.Path(),
					"offset":  y.Offset(),
					"line":    y.Line(),
					"column":  y.Column(),
					"length":  y.Length(),
					"message": y.Message(),
					// "source":  y.Source(),
				})
			}
		}
		printResult(combined)
		os.Exit(code)
	}

	printWarnings(warnings)
	if len(errs) > 0 {
		fmt.Fprintln(os.Stderr, "❌ Error while validating template:")
		for i, err := range errs {
			fmt.Fprintf(os.Stderr, "%v\n",
				colorMarkerRegex.ReplaceAllString(err.Error(), colorize(31, "$0")))
			if i < len(errs)-1 {
				fmt.Fprintln(os.Stderr, "")
			}
		}
	}

	os.Exit(code)
}

func mustLoad(filename string) *sdt.Document {
	doc, err := sdt.NewFromFile(filename)
	if err != nil {
		exitErr(1, "❌ Unable to load "+filename, err)
	}

	// Validate template output format
	warnings, errs := doc.ValidateTemplate()

	if len(errs) > 0 {
		exit(1, "❌ Error while validating template", warnings, errs)
	}

	printWarnings(warnings)

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

	root.PersistentFlags().StringVarP(&format, "output", "o", "default", "Output format [json, yaml, shorthand]")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

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
				exitErr(1, "❌ Error generating example\n", err)
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
				exitErr(1, "❌ Error getting input\n%v", err)
			}
			if params == nil {
				params = map[string]interface{}{}
			} else {
				// Temporary fix: round-trip to remove custom shorthand list types.
				enc, err := json.Marshal(params)
				if err != nil {
					panic(err)
				}
				json.Unmarshal(enc, &params)
			}

			if verbose {
				fmt.Fprintln(os.Stderr, "Input:")
				printResult(params)
			}

			// Validate params from template input schema
			if err := doc.ValidateInput(params); err != nil {
				exitErr(1, "❌ Error while validating input params:", err)
			}

			// Render the output
			rendered, errs := doc.Render(params)
			if len(errs) > 0 {
				exit(1, "❌ Error while rendering template:", nil, errs)
			}

			// Confirm that the output conforms to the schema now that it's rendered.
			if err := doc.ValidateOutput(rendered); err != nil {
				if verbose {
					fmt.Fprintln(os.Stderr, "Rendered result:")
					printResult(rendered)
				}
				exitErr(1, "❌ Error validating rendered output:", err)
			}

			if verbose {
				fmt.Fprintln(os.Stderr, "Result:")
			}
			printResult(rendered)
		},
	}

	root.AddCommand(example)
	root.AddCommand(validate)
	root.AddCommand(render)

	root.Execute()
}
