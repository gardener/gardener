// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package networkpolicies

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SourceBuilder test", func() {

	var (
		source  *SourcePod
		builder *RuleBuilder
	)

	BeforeEach(func() {
		source = &SourcePod{
			Pod:   NewPod("source", nil),
			Ports: NewSinglePort(80),
		}
	})
	JustBeforeEach(func() {
		builder = NewSource(source)
	})

	Context("deny and allow targetPods", func() {
		var (
			denyPod1 = &TargetPod{
				Pod:  NewPod("target-1", nil),
				Port: Port{Port: 8080},
			}
			denyPod2 = &TargetPod{
				Pod:  NewPod("target-2", nil),
				Port: Port{Port: 8080},
			}
			denyPod3 = &TargetPod{
				Pod:  NewPod("target-3", nil),
				Port: Port{Port: 8080},
			}
		)

		It("should accept multiple entries", func() {
			result := builder.DenyTargetPod(denyPod1, denyPod2).AllowTargetPod(denyPod2).Build()
			expected := Rule{source, []PodRule{
				{*denyPod1, false},
				{*denyPod2, true},
			}, nil}
			Expect(result).To(Equal(expected))
		})

		It("should accept multiple entries", func() {
			result := builder.DenyTargetPod(denyPod1, denyPod3, denyPod2).AllowTargetPod(denyPod1).Build()
			expected := Rule{source, []PodRule{
				{*denyPod1, true},
				{*denyPod3, false},
				{*denyPod2, false},
			}, nil}
			Expect(result).To(Equal(expected))
		})

		It("should accept multiple entries", func() {
			result := builder.AllowTargetPod(denyPod1, denyPod3, denyPod2).Build()
			expected := Rule{source, []PodRule{
				{*denyPod1, true},
				{*denyPod3, true},
				{*denyPod2, true},
			}, nil}
			Expect(result).To(Equal(expected))
		})

		Context("self action", func() {
			var (
				denySelf = &TargetPod{
					Pod:  NewPod("source", nil),
					Port: Port{Port: 80},
				}
			)
			It("should not add self", func() {
				result := builder.DenyTargetPod(denySelf).AllowTargetPod(denySelf).Build()
				expected := Rule{source, nil, nil}
				Expect(result).To(Equal(expected))
			})
			It("should not deny self", func() {
				result := builder.DenyTargetPod(denySelf).DenyTargetPod(denySelf).Build()
				expected := Rule{source, nil, nil}
				Expect(result).To(Equal(expected))
			})
		})
	})

	Context("deny and allow sourcePods", func() {
		var (
			denyTargetPod = &SourcePod{
				Pod:   NewPod("target", nil),
				Ports: []Port{{Port: 8080}, {Port: 8081}},
			}
			denyTargetPod2 = &SourcePod{
				Pod:   NewPod("target", nil),
				Ports: NewSinglePort(8081),
			}
			denyTargetPod3 = &SourcePod{
				Pod:   NewPod("target-2", nil),
				Ports: NewSinglePort(8080),
			}
			denyPod1 = &TargetPod{
				Pod:  NewPod("target", nil),
				Port: Port{Port: 8080},
			}
			denyPod2 = &TargetPod{
				Pod:  NewPod("target", nil),
				Port: Port{Port: 8081},
			}
			denyPod3 = &TargetPod{
				Pod:  NewPod("target-2", nil),
				Port: Port{Port: 8080},
			}
		)

		It("should accept multiple entries", func() {
			result := builder.DenyPod(denyTargetPod).AllowPod(denyTargetPod2).Build()
			expected := Rule{source, []PodRule{
				{*denyPod1, false},
				{*denyPod2, true},
			}, nil}
			Expect(result).To(Equal(expected))
		})

		It("should accept multiple entries", func() {
			result := builder.DenyPod(denyTargetPod, denyTargetPod3).AllowPod(denyTargetPod).Build()
			expected := Rule{source, []PodRule{
				{*denyPod1, true},
				{*denyPod2, true},
				{*denyPod3, false},
			}, nil}
			Expect(result).To(Equal(expected))
		})

		It("should accept multiple entries", func() {
			result := builder.AllowPod(denyTargetPod, denyTargetPod3).Build()
			expected := Rule{source, []PodRule{
				{*denyPod1, true},
				{*denyPod2, true},
				{*denyPod3, true},
			}, nil}
			Expect(result).To(Equal(expected))
		})

		Context("self action", func() {
			var (
				denySelf = &TargetPod{
					Pod:  NewPod("source", nil),
					Port: Port{Port: 80},
				}
			)
			It("should not add self", func() {
				result := builder.DenyTargetPod(denySelf).AllowTargetPod(denySelf).Build()
				expected := Rule{source, nil, nil}
				Expect(result).To(Equal(expected))
			})
			It("should not deny self", func() {
				result := builder.DenyTargetPod(denySelf).DenyTargetPod(denySelf).Build()
				expected := Rule{source, nil, nil}
				Expect(result).To(Equal(expected))
			})
		})
	})

	Context("deny and allow targetHost", func() {
		var (
			denyHost1 = &Host{
				HostName: "foo.bar",
				Port:     8080,
			}
			denyHost2 = &Host{
				HostName: "bar.baz",
				Port:     8082,
			}
			denyHost3 = &Host{
				HostName: "foo.baz",
				Port:     8083,
			}
		)

		It("should accept multiple entries", func() {
			result := builder.DenyHost(denyHost1, denyHost2).AllowHost(denyHost2).Build()
			expected := Rule{source, nil, []HostRule{
				{*denyHost1, false},
				{*denyHost2, true},
			}}
			Expect(result).To(Equal(expected))
		})

		It("should accept multiple entries", func() {
			result := builder.DenyHost(denyHost1, denyHost3, denyHost2).AllowHost(denyHost1).Build()
			expected := Rule{source, nil, []HostRule{
				{*denyHost1, true},
				{*denyHost3, false},
				{*denyHost2, false},
			}}
			Expect(result).To(Equal(expected))
		})

		It("should accept multiple entries", func() {
			result := builder.AllowHost(denyHost1, denyHost3, denyHost2).Build()
			expected := Rule{source, nil, []HostRule{
				{*denyHost1, true},
				{*denyHost3, true},
				{*denyHost2, true},
			}}
			Expect(result).To(Equal(expected))
		})
	})
})
