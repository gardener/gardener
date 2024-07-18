// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package structuredmap

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Structured Map", func() {
	Describe("#SetMapEntry", func() {
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

			got, err := SetMapEntry(m, Path{"a", "b", "c"}, func(_ any) (any, error) { return 2, nil })
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(want))
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

			_, err := SetMapEntry(m, Path{"a", "b", "d"}, func(_ any) (any, error) { return map[string]any{"e": 2}, nil })
			Expect(err).NotTo(HaveOccurred())

			got, err := SetMapEntry(m, Path{"a", "b"}, func(val any) (any, error) {
				values, ok := val.(map[string]any)
				if !ok {
					values = map[string]any{}
				}
				values["c"] = 2

				return values, nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(want))
		})

		It("should populate a nil map", func() {
			var (
				m    map[string]any
				want = map[string]any{
					"a": map[string]any{
						"b": map[string]any{
							"c": true,
						},
					},
				}
			)

			got, err := SetMapEntry(m, Path{"a", "b", "c"}, func(_ any) (any, error) { return true, nil })
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(want))
		})

		It("should return an error when there are no path elements", func() {
			got, err := SetMapEntry(nil, Path{}, func(_ any) (any, error) { return true, nil })
			Expect(err).To(MatchError(fmt.Errorf("at least one path element for patching is required")))
			Expect(got).To(BeNil())
		})

		It("should return an error when no setter function is given", func() {
			got, err := SetMapEntry(nil, Path{"a"}, nil)
			Expect(err).To(MatchError(fmt.Errorf("setter function must not be nil")))
			Expect(got).To(BeNil())
		})

		It("should return an error when the setter function returns an error", func() {
			got, err := SetMapEntry(nil, Path{"a", "b"}, func(_ any) (any, error) { return nil, fmt.Errorf("unable to set value") })
			Expect(err).To(MatchError(fmt.Errorf("unable to set value")))
			Expect(got).To(BeNil())
		})

		It("traversing into a non-map turns into an error", func() {
			got, err := SetMapEntry(map[string]any{"a": true}, Path{"a", "b", "c"}, func(_ any) (any, error) { return nil, nil })
			Expect(err).To(MatchError(fmt.Errorf(`unable to traverse into data structure because value at "a" is not a map`)))
			Expect(got).To(BeNil())
		})
	})
})
