// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package structuredmap

import (
	"fmt"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Structured Map", func() {
	Describe("#SetMapEntry", func() {
		It("should do nothing when map is nil", func() {
			var m map[string]any

			Expect(SetMapEntry(m, Path{"a", "b"}, func(_ any) (any, error) { return true, nil })).To(Succeed())
			Expect(m).To(BeNil())
		})

		It("should set the value in an existing map", func() {
			var (
				m = map[string]any{
					"a": map[string]any{
						"b": map[string]any{
							"c": 1,
						},
					},
				}
				want = map[string]any{
					"a": map[string]any{
						"b": map[string]any{
							"c": 2,
						},
					},
				}
			)

			Expect(SetMapEntry(m, Path{"a", "b", "c"}, func(_ any) (any, error) { return 2, nil })).To(Succeed())
			Expect(m).To(Equal(want))
		})

		It("should set values when calling multiple times", func() {
			var (
				m    = map[string]any{}
				want = map[string]any{
					"a": map[string]any{
						"b": map[string]any{
							"c": 2,
							"d": map[string]any{
								"e": 2,
							},
						},
					},
				}
			)

			Expect(SetMapEntry(m, Path{"a", "b", "d"}, func(_ any) (any, error) { return map[string]any{"e": 2}, nil })).To(Succeed())

			Expect(SetMapEntry(m, Path{"a", "b"}, func(val any) (any, error) {
				values, ok := val.(map[string]any)
				if !ok {
					values = map[string]any{}
				}
				values["c"] = 2

				return values, nil
			})).To(Succeed())
			Expect(m).To(Equal(want))
		})

		It("should populate a nil map", func() {
			var (
				m    = make(map[string]any)
				want = map[string]any{
					"a": map[string]any{
						"b": map[string]any{
							"c": true,
						},
					},
				}
			)

			Expect(SetMapEntry(m, Path{"a", "b", "c"}, func(_ any) (any, error) { return true, nil })).To(Succeed())
			Expect(m).To(Equal(want))
		})

		It("should return an error when there are no path elements", func() {
			m := make(map[string]any)

			Expect(SetMapEntry(m, Path{}, func(_ any) (any, error) { return true, nil })).To(MatchError(fmt.Errorf("at least one path element for patching is required")))
			Expect(m).To(BeEmpty())
		})

		It("should return an error when no setter function is given", func() {
			m := make(map[string]any)

			Expect(SetMapEntry(nil, Path{"a"}, nil)).To(MatchError(fmt.Errorf("setter function must not be nil")))
			Expect(m).To(BeEmpty())
		})

		It("should return an error when the setter function returns an error", func() {
			m := make(map[string]any)

			Expect(SetMapEntry(m, Path{"a", "b"}, func(_ any) (any, error) { return nil, fmt.Errorf("unable to set value") })).To(MatchError(fmt.Errorf("unable to set value")))
			Expect(m).To(BeEmpty())
		})

		It("traversing into a non-map turns into an error", func() {
			var (
				m     = map[string]any{"a": true}
				mCopy = make(map[string]any)
			)
			maps.Copy(mCopy, m)

			Expect(SetMapEntry(m, Path{"a", "b", "c"}, func(_ any) (any, error) { return nil, nil })).To(MatchError(fmt.Errorf(`unable to traverse into data structure because value at "a" is not a map`)))
			Expect(m).To(Equal(mCopy))
		})
	})
})
