// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	"errors"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/apis/utils"
)

var _ = Describe("#WalkStructure", func() {
	identity := func(s string) (any, error) { return s, nil }

	It("should return the input unchanged when visit is the identity", func() {
		input := map[string]any{
			"a": "b",
			"nested": map[string]any{
				"c": "d",
				"list": []any{
					"x",
					map[string]any{"k": "v"},
					42,
					true,
					nil,
				},
			},
		}

		result, err := WalkStructure(input, identity)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(input))
	})

	It("should leave non-string scalars untouched", func() {
		input := map[string]any{
			"int":    42,
			"float":  3.14,
			"bool":   true,
			"nil":    nil,
			"string": "x",
		}
		visited := []string{}
		visit := func(s string) (any, error) {
			visited = append(visited, s)
			return s, nil
		}

		result, err := WalkStructure(input, visit)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(input))
		// Visited keys + the single string value.
		Expect(visited).To(ConsistOf("int", "float", "bool", "nil", "string", "x"))
	})

	It("should transform string values", func() {
		input := map[string]any{
			"a": "hello",
			"nested": map[string]any{
				"b": "world",
			},
		}

		result, err := WalkStructure(input, func(s string) (any, error) {
			return strings.ToUpper(s), nil
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"A": "HELLO",
			"NESTED": map[string]any{
				"B": "WORLD",
			},
		}))
	})

	It("should walk strings inside slices", func() {
		input := map[string]any{
			"list": []any{"a", "b", []any{"c"}},
		}

		result, err := WalkStructure(input, func(s string) (any, error) {
			return s + s, nil
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(map[string]any{
			"listlist": []any{"aa", "bb", []any{"cc"}},
		}))
	})

	It("should allow visit to return non-string values", func() {
		input := map[string]any{"value": "42"}

		result, err := WalkStructure(input, func(s string) (any, error) {
			if s == "42" {
				return 42, nil
			}
			return s, nil
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(map[string]any{"value": 42}))
	})

	It("should propagate errors from visit", func() {
		boom := errors.New("boom")
		input := map[string]any{"a": "b"}

		_, err := WalkStructure(input, func(string) (any, error) {
			return nil, boom
		})
		Expect(err).To(MatchError(ContainSubstring("boom")))
	})

	It("should return an error when visit on a map key returns a non-string", func() {
		input := map[string]any{"a": "b"}

		_, err := WalkStructure(input, func(string) (any, error) {
			return 1, nil
		})
		Expect(err).To(MatchError(ContainSubstring("expected string after processing map key")))
	})

	It("should return an error when two map keys collide after visiting", func() {
		input := map[string]any{
			"foo": "1",
			"bar": "2",
		}

		_, err := WalkStructure(input, func(string) (any, error) {
			return "same", nil
		})
		Expect(err).To(MatchError(ContainSubstring(`duplicate map key "same"`)))
	})

	It("should return the input unchanged when given a string", func() {
		result, err := WalkStructure("hello", identity)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal("hello"))
	})

	It("should return the input unchanged when given a non-string scalar", func() {
		result, err := WalkStructure(42, identity)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(42))
	})

	It("should return nil unchanged", func() {
		result, err := WalkStructure(nil, identity)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(BeNil())
	})

	It("should handle empty maps and slices", func() {
		result, err := WalkStructure(map[string]any{}, identity)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal(map[string]any{}))

		result, err = WalkStructure([]any{}, identity)
		Expect(err).ToNot(HaveOccurred())
		Expect(result).To(Equal([]any{}))
	})
})
