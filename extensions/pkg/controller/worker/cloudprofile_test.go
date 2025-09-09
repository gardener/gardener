// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

type ProviderCapabilitySet struct {
	Name         string
	Capabilities v1beta1.Capabilities
}

func (t ProviderCapabilitySet) GetCapabilities() v1beta1.Capabilities {
	return t.Capabilities
}

var _ = Describe("Worker CloudProfile helper", func() {
	Describe("#FindBestCapabilitySet", func() {
		var (
			capabilityDefinitions []v1beta1.CapabilityDefinition
			capabilitySets        []ProviderCapabilitySet
		)

		BeforeEach(func() {
			capabilityDefinitions = []v1beta1.CapabilityDefinition{
				{
					Name:   "architecture",
					Values: []string{"amd64", "arm64"},
				},
				{
					Name:   "foo",
					Values: []string{"bar", "baz", "qux", "xxx"},
				},
			}

			capabilitySets = []ProviderCapabilitySet{
				{
					Name: "amd64-set",
					Capabilities: v1beta1.Capabilities{
						"foo":          []string{"bar", "qux"},
						"architecture": []string{"amd64"},
					},
				},
				{
					Name: "amd64-set-2",
					Capabilities: v1beta1.Capabilities{
						"foo":          []string{"bar", "baz"},
						"architecture": []string{"amd64"},
					},
				},
				{
					Name: "arm64-set",
					Capabilities: v1beta1.Capabilities{
						"foo":          []string{"bar", "baz", "qux"},
						"architecture": []string{"arm64"},
					},
				},
			}
		})

		It("should find an exact matching capability set", func() {
			// Request only matches first capability set
			requestedCapabilities := v1beta1.Capabilities{
				"architecture": []string{"amd64"},
				"foo":          []string{"qux"},
			}

			result, err := worker.FindBestCapabilitySet(capabilitySets, requestedCapabilities, capabilityDefinitions)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Name).To(Equal("amd64-set"))
		})

		It("should find best match based on capability priorities", func() {
			// Both sets are compatible, but the first one is preferred due to "amd64" having higher priority
			requestedCapabilities := v1beta1.Capabilities{
				"foo": []string{"qux"},
			}

			result, err := worker.FindBestCapabilitySet(capabilitySets, requestedCapabilities, capabilityDefinitions)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Name).To(Equal("amd64-set"))
		})

		It("should return error when no compatible capability set is found", func() {
			// Requested capabilities not compatible with any set
			requestedCapabilities := v1beta1.Capabilities{
				"architecture": []string{"arm64"},
				"foo":          []string{"xxx"}, // arm64 set only has "bar" "baz" and "qux"
			}

			_, err := worker.FindBestCapabilitySet(capabilitySets, requestedCapabilities, capabilityDefinitions)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no compatible capability set found"))
		})

		It("should find the most appropriate set based on capability value preferences", func() {
			requestedCapabilities := v1beta1.Capabilities{
				"architecture": []string{"amd64"},
				"foo":          []string{"bar"},
			}

			result, err := worker.FindBestCapabilitySet(capabilitySets, requestedCapabilities, capabilityDefinitions)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Name).To(Equal("amd64-set-2"))
		})

		It("should prioritize capabilities based on their order in definitions", func() {
			// Reorder definitions to prioritize "foo" over "architecture"
			reorderedDefinitions := []v1beta1.CapabilityDefinition{
				{
					Name:   "foo",
					Values: []string{"bar", "baz", "qux"},
				},
				{
					Name:   "architecture",
					Values: []string{"amd64", "arm64"},
				},
			}

			requestedCapabilities := v1beta1.Capabilities{
				"foo": []string{"bar"},
			}

			result, err := worker.FindBestCapabilitySet(capabilitySets, requestedCapabilities, reorderedDefinitions)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Name).To(Equal("arm64-set")) // "baz" has higher preference in foo values
		})

		It("should handle capability sets with multiple values", func() {
			multiValueSets := []*ProviderCapabilitySet{
				{
					Name: "multi-arch-set",
					Capabilities: v1beta1.Capabilities{
						"architecture": []string{"amd64"},
						"foo":          []string{"bar", "baz"},
					},
				},
				{
					Name: "single-arch-set",
					Capabilities: v1beta1.Capabilities{
						"architecture": []string{"amd64"},
						"foo":          []string{"bar", "baz", "qux"},
					},
				},
			}

			requestedCapabilities := v1beta1.Capabilities{
				"architecture": []string{"amd64"},
				"foo":          []string{"bar"},
			}

			result, err := worker.FindBestCapabilitySet(multiValueSets, requestedCapabilities, capabilityDefinitions)

			Expect(err).NotTo(HaveOccurred())
			Expect(result.Name).To(Equal("single-arch-set")) // More specific architecture match is preferred
		})
	})
})
