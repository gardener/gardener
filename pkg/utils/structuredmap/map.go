// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package structuredmap

import (
	"fmt"
)

type (
	// Path is the path to an element in a structured map.
	Path []string
	// SetFn is the function to set a config element to the given value.
	SetFn func(value any) (any, error)
)

// SetMapEntry sets an entry in the given map. It invokes the setter function at the given path with the respective value (if set).
func SetMapEntry(m map[string]any, path Path, setFn SetFn) error {
	if setFn == nil {
		return fmt.Errorf("setter function must not be nil")
	}
	if len(path) == 0 {
		return fmt.Errorf("at least one path element for patching is required")
	}

	return setMapEntry(m, path, setFn)
}

func setMapEntry(m map[string]any, path Path, setFn SetFn) error {
	if m == nil {
		return nil
	}

	var (
		key = path[0]
	)

	if len(path) == 1 {
		value := m[key]

		var err error
		m[key], err = setFn(value)
		if err != nil {
			return fmt.Errorf("error setting value: %w", err)
		}

		return nil
	}

	entry, ok := m[key]
	if !ok {
		entry = map[string]any{}
	}

	childMap, ok := entry.(map[string]any)
	if !ok {
		return fmt.Errorf("unable to traverse into data structure because value at %q is not a map", key)
	}

	if err := setMapEntry(childMap, path[1:], setFn); err != nil {
		return err
	}

	m[key] = childMap
	return nil
}
