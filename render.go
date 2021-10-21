package sdt

import (
	"fmt"
	"reflect"
	"regexp"

	"github.com/antonmedv/expr"
)

// This is the regex used to find/replace ${...} expressions within strings.
var interpolationRe = regexp.MustCompile(`[$][{].*?[}]`)

func handleBranch(ctx *context, v map[string]interface{}, params map[string]interface{}) interface{} {
	result := v["$if"]
	if s, ok := result.(string); ok {
		result = handleInterpolation(ctx, s, params)
	}

	if result != nil && !reflect.ValueOf(result).IsZero() {
		return render(ctx, v["$then"], params)
	} else if v["$else"] != nil {
		return render(ctx, v["$else"], params)
	}
	return nil
}

func handleLoop(ctx *context, v map[string]interface{}, params map[string]interface{}) interface{} {
	items := v["$for"]

	if itemsExpr, ok := items.(string); ok {
		items = handleInterpolation(ctx, itemsExpr, params)
	}

	if items == nil {
		return nil
	}

	if items, ok := items.([]interface{}); ok {
		tmp := []interface{}{}

		for i, item := range items {
			paramsCopy := map[string]interface{}{}
			for k, v := range params {
				paramsCopy[k] = v
			}

			itemName := "item"
			if v["$as"] != nil {
				itemName = v["$as"].(string)
			}
			paramsCopy[itemName] = item

			loopName := "loop"
			if itemName != "item" {
				loopName += "_" + itemName
			}
			paramsCopy[loopName] = map[string]interface{}{
				"index": i,
				"first": i == 0,
				"last":  i == len(items)-1,
			}

			itemResult := render(ctx.WithPath(i), v["$each"], paramsCopy)
			tmp = append(tmp, itemResult)
		}

		return tmp
	}

	return ctx.AddError(fmt.Errorf("error rendering: $for expression result is not iterable: %v", items))
}

func handleFlatten(ctx *context, v map[string]interface{}, params map[string]interface{}) interface{} {
	result := render(ctx.WithPath("$flatten"), v["$flatten"], params)

	if s, ok := result.([]interface{}); ok {
		tmp := make([]interface{}, 0, len(s))
		for _, items := range s {
			tmp = append(tmp, items.([]interface{})...)
		}
		return tmp
	}

	return ctx.AddError(fmt.Errorf("error rendering: $flatten result is not iterable"))
}

func handleInterpolation(ctx *context, v string, params map[string]interface{}) interface{} {
	// Special case: full replacement; Could by any type, not just str so we
	// can't replace by strings and instead just return the one value from the
	// expression given the current context.
	matches := interpolationRe.FindAllString(v, -1)
	if len(matches) == 1 && len(matches[0]) == len(v) {
		result, err := expr.Eval(v[2:len(v)-1], params)
		if err != nil {
			return ctx.AddError(fmt.Errorf("error rendering: %w", err))
		}
		return result
	}

	// Everything else generates a string as output.
	interpolated := interpolationRe.ReplaceAllStringFunc(v, func(v string) string {
		result, err := expr.Eval(v[2:len(v)-1], params)
		if err != nil {
			ctx.AddError(fmt.Errorf("error rendering: %w", err))
			return ""
		}
		if result != nil {
			return fmt.Sprintf("%v", result)
		}
		return ""
	})

	return interpolated
}

func render(ctx *context, template interface{}, params map[string]interface{}) interface{} {
	switch v := template.(type) {
	case map[string]interface{}:
		// This is an object in the template. First, handle special syntax for
		// branching/looping/etc, then if none of those are present, fall back
		// to normal key/value recursive processing.
		if v["$if"] != nil {
			return handleBranch(ctx, v, params)
		}
		if v["$for"] != nil {
			return handleLoop(ctx, v, params)
		}
		if v["$flatten"] != nil {
			return handleFlatten(ctx, v, params)
		}

		tmp := map[string]interface{}{}
		for k, v := range v {
			kr := render(ctx.WithPath(k), k, params)
			if krs, ok := kr.(string); ok {
				vr := render(ctx.WithPath(k), v, params)
				if vr != nil {
					tmp[krs] = vr
				}
			}
		}
		return tmp
	case []interface{}:
		tmp := []interface{}{}

		for i, item := range v {
			result := render(ctx.WithPath(i), item, params)
			if result != nil {
				tmp = append(tmp, result)
			}
		}

		return tmp
	case string:
		return handleInterpolation(ctx, v, params)
	}

	return template
}
