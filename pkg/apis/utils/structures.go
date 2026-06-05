// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
)

// WalkStructure walks the given JSON-like structure of nested `map[string]any` and `[]any` values,
// invoking `visit` for every string (both map keys and values). The result has the same shape as
// the input with strings replaced by the value returned from `visit`. Non-string scalars are left
// untouched. When a map key visit returns a non-string value or two keys collide after visiting,
// an error is returned. The walk aborts and returns the first error returned by `visit`.
func WalkStructure(in any, visit func(string) (any, error)) (any, error) {
	switch v := in.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, val := range v {
			keyResult, err := visit(k)
			if err != nil {
				return nil, fmt.Errorf("failed to process map key %q: %w", k, err)
			}
			newKey, ok := keyResult.(string)
			if !ok {
				return nil, fmt.Errorf("expected string after processing map key %q, got %T", k, keyResult)
			}
			res, err := WalkStructure(val, visit)
			if err != nil {
				return nil, err
			}
			if _, exists := out[newKey]; exists {
				return nil, fmt.Errorf("duplicate map key %q after visiting", newKey)
			}
			out[newKey] = res
		}
		return out, nil
	case []any:
		out := make([]any, len(v))
		for i, val := range v {
			res, err := WalkStructure(val, visit)
			if err != nil {
				return nil, err
			}
			out[i] = res
		}
		return out, nil
	case string:
		return visit(v)
	default:
		return v, nil
	}
}
