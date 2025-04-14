// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	"net"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/gardener/operator"
)

var _ = Describe("utils", func() {
	Describe("#MergeStringMaps", func() {
		It("should return an empty map", func() {
			emptyMap := map[string]string{}

			result := MergeStringMaps(emptyMap, nil)

			Expect(result).To(Equal(emptyMap))
		})

		It("should return a merged map (string value)", func() {
			var (
				oldMap = map[string]string{
					"a": "1",
					"b": "2",
				}
				newMap = map[string]string{
					"b": "20",
					"c": "3",
				}
			)

			result := MergeStringMaps(oldMap, newMap)

			Expect(result).To(Equal(map[string]string{
				"a": "1",
				"b": "20",
				"c": "3",
			}))
		})

		It("should return a merged map (bool value)", func() {
			var (
				a = map[string]bool{
					"p": true,
					"q": false,
				}
				b = map[string]bool{
					"q": true,
					"r": false,
				}
			)

			result := MergeStringMaps(a, b)

			Expect(result).To(Equal(map[string]bool{
				"p": true,
				"q": true,
				"r": false,
			}))
		})
	})

	DescribeTable("#IDForKeyWithOptionalValue",
		func(key string, value *string, expectation string) {
			Expect(IDForKeyWithOptionalValue(key, value)).To(Equal(expectation))
		},
		Entry("only key", "foo", nil, "foo"),
		Entry("key and value", "foo", ptr.To("bar"), "foo=bar"),
	)

	Describe("#Indent", func() {
		var spaces = 2

		It("should not indent a single-line string", func() {
			Expect(Indent("foo", spaces)).To(Equal("foo"))
		})

		It("should properly indent a multi-line string", func() {
			Expect(Indent(`foo
bar
baz`, spaces)).To(Equal(`foo
  bar
  baz`))
		})
	})

	Describe("#ShallowCopyMapStringInterface", func() {
		It("should create a shallow copy of the map", func() {
			v := map[string]any{"foo": nil, "bar": map[string]any{"baz": nil}}

			c := ShallowCopyMapStringInterface(v)

			Expect(c).To(Equal(v))

			c["foo"] = 1
			Expect(v["foo"]).To(BeNil())

			c["bar"].(map[string]any)["baz"] = "bang"
			Expect(v["bar"].(map[string]any)["baz"]).To(Equal("bang"))
		})
	})

	DescribeTable("#IifString",
		func(condition bool, expectation string) {
			Expect(IifString(condition, "true", "false")).To(Equal(expectation))
		},
		Entry("condition is true", true, "true"),
		Entry("condition is false", false, "false"),
	)

	Describe("#InterfaceMapToStringMap", func() {
		input := map[string]any{
			"foo":   nil,
			"age":   32,
			"alive": true,
			"name":  "haralampi",
		}

		output := map[string]string{
			"foo":   "<nil>",
			"age":   "32",
			"alive": "true",
			"name":  "haralampi",
		}

		It("should return map[string]string", func() {
			Expect(InterfaceMapToStringMap(input)).Should(Equal(output))
		})
	})

	Describe("#ComputeOffsetIP", func() {
		Context("IPv4", func() {
			It("should return a cluster IPv4 IP", func() {
				_, subnet, _ := net.ParseCIDR("100.64.0.0/13")
				result, err := ComputeOffsetIP(subnet, 10)

				Expect(err).NotTo(HaveOccurred())

				Expect(result).To(HaveLen(net.IPv4len))
				Expect(result).To(Equal(net.ParseIP("100.64.0.10").To4()))
			})

			It("should return error if subnet nil is passed", func() {
				result, err := ComputeOffsetIP(nil, 10)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should return error if subnet is not big enough is passed", func() {
				_, subnet, _ := net.ParseCIDR("100.64.0.0/32")
				result, err := ComputeOffsetIP(subnet, 10)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should return error if ip address is broadcast ip", func() {
				_, subnet, _ := net.ParseCIDR("10.0.0.0/24")
				result, err := ComputeOffsetIP(subnet, 255)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})

		Context("IPv6", func() {
			It("should return a cluster IPv6 IP", func() {
				_, subnet, _ := net.ParseCIDR("fc00::/8")
				result, err := ComputeOffsetIP(subnet, 10)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(HaveLen(net.IPv6len))
				Expect(result).To(Equal(net.ParseIP("fc00::a")))
			})

			It("should return error if subnet nil is passed", func() {
				result, err := ComputeOffsetIP(nil, 10)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})

			It("should return error if subnet is not big enough is passed", func() {
				_, subnet, _ := net.ParseCIDR("fc00::/128")
				result, err := ComputeOffsetIP(subnet, 10)

				Expect(err).To(HaveOccurred())
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("#FilterEntriesByFilterFn", func() {
		var (
			entries  []string
			filterFn func(string) bool
		)

		BeforeEach(func() {
			entries = []string{
				"foo.bash",
				"bar.bash",
				"boo.dash",
				"zig.zag",
			}

			filterFn = nil
		})

		It("should return all entries when filter function is nil", func() {
			result := FilterEntriesByFilterFn(entries, filterFn)
			Expect(result).To(Equal(entries))
		})

		It("should return no entries", func() {
			result := FilterEntriesByFilterFn(nil, filterFn)
			Expect(result).To(BeEmpty())
		})

		It("should only return entries matching the filter function", func() {
			filterFn = func(entry string) bool {
				return strings.HasSuffix(entry, "bash")
			}

			result := FilterEntriesByFilterFn(entries, filterFn)
			Expect(result).To(ConsistOf(
				"foo.bash",
				"bar.bash",
			))
		})

		It("should only return entries matching the filter function", func() {
			filterFn = func(entry string) bool {
				return !strings.HasSuffix(entry, "bash")
			}

			result := FilterEntriesByFilterFn(entries, filterFn)
			Expect(result).To(ConsistOf(
				"boo.dash",
				"zig.zag",
			))
		})

		It("should only return entries matching the filter function", func() {
			entries = []string{
				"secrets",
				"configmaps",
				"shoots.core.gardener.cloud",
				"bastions.operations.gardener.cloud",
			}
			filterFn = func(s string) bool {
				return !operator.IsServedByGardenerAPIServer(s)
			}

			result := FilterEntriesByFilterFn(entries, filterFn)
			Expect(result).To(ConsistOf(
				"secrets",
				"configmaps",
			))
		})

		It("should only return entries matching the filter function", func() {
			entries = []string{
				"secrets",
				"configmaps",
				"shoots.core.gardener.cloud",
				"bastions.operations.gardener.cloud",
			}

			result := FilterEntriesByFilterFn(entries, operator.IsServedByGardenerAPIServer)
			Expect(result).To(ConsistOf(
				"shoots.core.gardener.cloud",
				"bastions.operations.gardener.cloud",
			))
		})
	})

	Describe("#CreateMapFromSlice", func() {
		type entry struct {
			name  string
			value int
		}

		It("should correctly convert an empty slice", func() {
			var entries []string
			keyFunc := func(s string) string { return s }
			result := CreateMapFromSlice(entries, keyFunc)
			Expect(result).To(Equal(map[string]string{}))
		})

		It("should return an empty map for a nil keyFunc", func() {
			entries := []string{"a", "b", "c"}
			var keyFunc func(string) string = nil
			result := CreateMapFromSlice(entries, keyFunc)
			Expect(result).To(Equal(map[string]string{}))
		})

		It("should correctly create a map with a valid keyFunc returning string", func() {
			entries := []entry{{name: "a", value: 7}, {name: "b", value: 14}}
			keyFunc := func(e entry) string { return e.name }
			result := CreateMapFromSlice(entries, keyFunc)
			Expect(result).To(Equal(map[string]entry{
				"a": {name: "a", value: 7},
				"b": {name: "b", value: 14},
			}))
		})

		It("should correctly create a map with a valid keyFunc returning int", func() {
			entries := []entry{{name: "a", value: 7}, {name: "b", value: 14}}
			keyFunc := func(e entry) int { return e.value }
			result := CreateMapFromSlice(entries, keyFunc)
			Expect(result).To(Equal(map[int]entry{
				7:  {name: "a", value: 7},
				14: {name: "b", value: 14},
			}))
		})
	})
})
