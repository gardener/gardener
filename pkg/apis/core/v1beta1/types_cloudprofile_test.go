package v1beta1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("CloudProfile types", func() {
	Describe("CapabilityValues", func() {
		Describe("#UnmarshalJSON", func() {
			It("should successfully unmarshal from JSON", func() {
				values := v1beta1.CapabilityValues{}

				Expect(values.UnmarshalJSON([]byte(`["amd64", "arm64"]`))).To(Succeed())
				Expect(values.Values).To(ConsistOf("amd64", "arm64"))
			})

			It("should fail to unmarshal from JSON", func() {
				values := v1beta1.CapabilityValues{}

				Expect(values.UnmarshalJSON([]byte(`"amd64", "arm64"`))).To(HaveOccurred())
				Expect(values.Values).To(BeEmpty())
			})
		})
	})

	Describe("CapabilitySets", func() {
		Describe("#UnmarshalJSON", func() {
			It("should successfully unmarshal from JSON", func() {
				values := v1beta1.CapabilitySet{}

				Expect(values.UnmarshalJSON([]byte(`{"architecture": ["amd64", "arm64"]}`))).To(Succeed())
				Expect(values.Capabilities["architecture"].Values).To(ConsistOf("amd64", "arm64"))
			})

			It("should fail to unmarshal from JSON", func() {
				values := v1beta1.CapabilitySet{}

				Expect(values.UnmarshalJSON([]byte(`{"architecture": {"values": amd64,arm64}}`))).To(HaveOccurred())
				Expect(values.Capabilities).To(BeEmpty())
			})
		})
	})
})
