# Structured Data Templates

Structured data templates are a templating engine that takes a simplified set of input parameters and transforms them into a complex structured data output. Both the inputs and outputs can be validated against a schema.

The goals of this project are to:

1. Provide a simple format: it's just JSON/YAML!
2. Give enough tools to be useful:
   - Interpolation `${my_value}` & `${num / 2 >= 5}`
   - Branching (if/then/else)
   - Looping (for/each)
3. Guarantee structural correctness
   - The structured data template is valid JSON / YAML
   - The input parameters are valid JSON / YAML
   - The output of the template is guaranteed to produce valid JSON
4. Provide tools for semantic correctness via schemas
   - The input types and values pass the schema
   - The template will produce output that should pass the schema
   - The output of the template after rendering passes the schema

## Structure

A structured data template document is made of two parts: schemas and a template. The schemas define the allowable input/output structure while the template defines the actual rendered output. An example document might look like:

```yaml
schemas:
  # Dialect selects the default JSON Schema version
  dialect: openapi-3.1
  input:
    # Input schema goes here
    type: object
    properties:
      name:
        type: string
        default: world
  output:
    # Output schema goes here, also supports refs:
    $ref: https://api.example.com/openapi.json#components/schemas/Greeting
template:
  # Templated output structure goes here
  greeting: Hello, ${name}!
```

## Example

You can run the example like so:

```sh
$ go run ./cmd/sdt ./samples/greeting.yaml <./samples/params.yaml
{
  "greeting": "Hello, SDT!"
}
```

Input params for rendering can be passed via stdin as JSON/YAML and/or via command line arguments as [CLI shorthand syntax](https://github.com/danielgtaylor/shorthand#readme).

## Schemas

JSON Schema is used for all schemas. It defaults to JSON Schema 2020-12 but can be overridden via the `$schema` key or using `dialect` in the structured data template document like above. Available dialects:

- `openapi-3.0`
- `openapi-3.1`
- `https://json-schema.org/draft/2020-12/schema`
- `https://json-schema.org/draft/2019-09/schema`
- `https://json-schema.org/draft-07/schema`
- `https://json-schema.org/draft-06/schema`
- `https://json-schema.org/draft-04/schema`

The input schema describes the input parameters and the template will not render unless the passed parameters validate using the input schema. It also lets you set defaults for the input parameters, which default to `nil` if not passed.

The output schema describes the template's output structure. The validator is capable of understanding branches & loops to ensure that the output is semantically valid regardless of which path is taken during rendering.

## Template Language Specification

A template is just JSON/YAML. For example:

```yaml
hello: world
```

That is a valid static template. Nothing will change when rendered, which is not very useful. Normally, when a template is rendered, it is passed parameters, and these are used for interpolation, branching, and looping. These features all make use of a basic expression language.

### Expressions

String interpolation, branching conditions, and loop variable selection all use an expression language. This allows you to make simple comparisons of the parameter context data. Examples:

- `foo > 50`
- `len(item.bars) <= 5 || my_override`
- `name contains "sdt"`
- `name startsWith "sdt"`
- `"foo" in ["foo", "bar"]`
- `loop.index + 1`

See [antonmedv/expr language definition](https://github.com/antonmedv/expr/blob/master/docs/Language-Definition.md) for details.

### String Interpolation

String interpolation is the act of replacing the contents of `${...}` within strings, where `...` corresponds to an expression that makes use of input parameters. For example:

```yaml
hello: ${name}
```

If passed `{"name": "Alice"}` as parameters this would render:

```json
{
  "hello": "Alice"
}
```

Whenever the string is just one `${...}` statement it will use whatever type it evaluates to in the result, so you are not limited to just strings. If the expression result is `nil`, then the property/item is not included in the rendered output.

It's also possible to add static text or multiple interpolation expressions in a single value:

```yaml
hello: Greetings, ${name}!
```

Given the same input that would result in:

```json
{
  "hello": "Greetings, Alice!"
}
```

#### Tricks

- Force a string output by using more than one expression: `${my_number}${""}`

### Branching

Branching allows one of multiple paths to be followed in the template at rendering time based on the result of an expression. The special properties `$if`, `$then`, and `$else` are used for this. For example:

```yaml
foo:
  $if: ${value > 5}
  $then: I am big
  $else: I am small
```

If rendered with `{"value": 1}` the result will be:

```json
{
  "foo": "I am small"
}
```

Notice that the special properties are completely removed and replaced with the contents of either the `$then` or `$else` clauses. So while in the _template_ `foo` is an object, the end result is that `foo` is a string and would pass the output schema.

If the expression is false and no `$then` is given, then the property is removed from the result.

### Looping

Looping allows an array of inputs to be expanded into the rendered output using a per-item template. The `$for`, `$as`, and `$each` special properties are used for this. For example:

```yaml
squares:
  $for: ${numbers}
  $each: ${item * item}
```

If rendered with `{"numbers": [1, 2, 3]}` the result will be:

```json
{
  "squares": [2, 4, 9]
}
```

The `$as` property controls the name of the variable holding the current item, which defaults to `item`. A local variable `loop` is also set, which includes an `index`, and whether the item is the `first` or `last` in the array. If using `$as` then the `loop` variable is named `loop_` + the `$as` value. This allows nested loops to access both their own and outer scope's loop variables. For example:

```yaml
things:
  $for: ${things}
  $as: thing
  $each:
    id: ${loop_thing.index}-${thing.name}
    tags:
      $for: ${tags}
      $as: tag
      $each: ${loop_thing.index}-${loop_tag.index}-${tag}
```

Given:

```json
{
  "things": [{ "name": "Alice" }, { "name": "Bob" }],
  "tags": ["big", "small"]
}
```

You would get as output:

```json
{
  "things": [
    {
      "id": "0-Alice",
      "tags": ["0-0-big", "0-1-small"]
    },
    {
      "id": "1-Bob",
      "tags": ["1-0-big", "1-1-small"]
    }
  ]
}
```

#### Multiple Outputs

If the result of the `$each` template is an array, then each item of that array is individually appended to the overall result. This allows one input to generate multiple output entries in the final array.

```yaml
things:
  $for: ${things}
  $each:
    - name: ${item.name} 1
    - name: ${itme.name} 2
```

With the same input as above you'd get:

```json
{
  "things": [
    {
      "name": "Alice 1"
    },
    {
      "name": "Alice 2"
    },
    {
      "name": "Bob 1"
    },
    {
      "name": "Bob 2"
    }
  ]
}
```

If you need to create arrays of arrays, wrap it in another array to get around this behavior.

## Open Questions

1. Should we support macros? Could be done with `$ref` in the template, and we could add a top-level `macros` or `definitions` for document-local refs. They would be drop-in only, no calling with arguments, but would render based on the current params context.

2. Should `nil` results from interpolation be rendered in the final output? Example: `name: ${name}` and what if `name` is `nil`?

3. Support for constants? Values that should always be present in the params that can contain complex and reusable data for the template?
