package sdt

import (
	"encoding/json"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v5"
)

// setDefaults takes user-provided input and traverses it along with the input
// schema to determine if unset values exist which have a default that should
// be set, then sets them. Params are modified in-place.
func setDefaults(s *jsonschema.Schema, params map[string]interface{}) {
	for s.Ref != nil {
		s = s.Ref
	}

	for k, v := range s.Properties {
		for v.Ref != nil {
			v = v.Ref
		}
		if v.Default != nil {
			if _, ok := params[k]; !ok {
				// Handle arbitrary JSON numbers by converting to the closest Go
				// type so that expressions work as expected. E.g. a json.Number can't
				// be added to an int, so this fixes that.
				switch d := v.Default.(type) {
				case json.Number:
					if hasType(v, "integer") {
						di, _ := d.Int64()
						params[k] = di
					} else {
						df, _ := d.Float64()
						params[k] = df
					}
				default:
					params[k] = v.Default
				}
			}
		}

		switch pv := params[k].(type) {
		case map[string]interface{}:
			// This is a nested object.
			setDefaults(v, pv)
		case []interface{}:
			// This is an array
			// TODO handle arrays of arrays
			for _, item := range pv {
				if o, ok := item.(map[string]interface{}); ok {
					setDefaults(getItems(v), o)
				}
			}
		}
	}
}
