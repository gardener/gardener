// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils_test

import (
	"fmt"
	"net"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"

	. "github.com/gardener/gardener/pkg/utils"
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
		Entry("key and value", "foo", pointer.String("bar"), "foo=bar"),
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
			v := map[string]interface{}{"foo": nil, "bar": map[string]interface{}{"baz": nil}}

			c := ShallowCopyMapStringInterface(v)

			Expect(c).To(Equal(v))

			c["foo"] = 1
			Expect(v["foo"]).To(BeNil())

			c["bar"].(map[string]interface{})["baz"] = "bang"
			Expect(v["bar"].(map[string]interface{})["baz"]).To(Equal("bang"))
		})
	})

	DescribeTable("#IifString",
		func(condition bool, expectation string) {
			Expect(IifString(condition, "true", "false")).To(Equal(expectation))
		},
		Entry("condition is true", true, "true"),
		Entry("condition is false", false, "false"),
	)

	Describe("#QuantityPtr", func() {
		It("should return a pointer", func() {
			Expect(QuantityPtr(resource.MustParse("64Gi"))).Should(Equal(resource.NewQuantity(68719476736, resource.BinarySI)))
		})
	})

	Describe("#ProtocolPtr", func() {
		It("should return a pointer", func() {
			Expect(ProtocolPtr(corev1.ProtocolTCP)).Should(gstruct.PointTo(Equal(corev1.ProtocolTCP)))
		})
	})

	Describe("#IntStrPtrFromInt", func() {
		It("should return a pointer", func() {
			Expect(IntStrPtrFromInt(1234)).Should(gstruct.PointTo(Equal(intstr.FromInt(1234))))
		})
	})

	Describe("#IntStrPtrFromString", func() {
		It("should return a pointer", func() {
			Expect(IntStrPtrFromString("foo")).Should(gstruct.PointTo(Equal(intstr.FromString("foo"))))
		})
	})

	Describe("#TimePtr", func() {
		It("should return a pointer", func() {
			now := time.Now()
			Expect(TimePtr(now)).Should(gstruct.PointTo(Equal(now)))
		})
	})

	Describe("#TimePtrDeref", func() {
		now := time.Now()
		def := now.Add(-time.Second)

		It("should return the pointer", func() {
			Expect(TimePtrDeref(&now, def)).Should(Equal(now))
		})

		It("should return the default", func() {
			Expect(TimePtrDeref(nil, def)).Should(Equal(def))
		})
	})

	Describe("#InterfaceMapToStringMap", func() {
		input := map[string]interface{}{
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

	Describe("#FilterEntriesByPrefix", func() {
		var (
			prefix  string
			entries []string
		)

		BeforeEach(func() {
			prefix = "role"
			entries = []string{
				"foo",
				"bar",
			}
		})

		It("should only return entries with prefix", func() {
			expectedEntries := []string{
				fmt.Sprintf("%s-%s", prefix, "foo"),
				fmt.Sprintf("%s-%s", prefix, "bar"),
			}

			entries = append(entries, expectedEntries...)

			result := FilterEntriesByPrefix(prefix, entries)
			Expect(result).To(ContainElements(expectedEntries))
		})

		It("should return all entries", func() {
			expectedEntries := []string{
				fmt.Sprintf("%s-%s", prefix, "foo"),
				fmt.Sprintf("%s-%s", prefix, "bar"),
			}

			entries = expectedEntries

			result := FilterEntriesByPrefix(prefix, entries)
			Expect(result).To(ContainElements(expectedEntries))
		})

		It("should return no entries", func() {
			result := FilterEntriesByPrefix(prefix, entries)
			Expect(result).To(BeEmpty())
		})
	})
})
