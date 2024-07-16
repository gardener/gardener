// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"
)

var _ = Describe("helper", func() {
	DescribeTable("#ClusterAutoscalerRequired",
		func(pools []extensionsv1alpha1.WorkerPool, expected bool) {
			Expect(ClusterAutoscalerRequired(pools)).To(Equal(expected))
		},

		Entry("no pools", []extensionsv1alpha1.WorkerPool{}, false),
		Entry("min=max", []extensionsv1alpha1.WorkerPool{{
			Minimum: 1,
			Maximum: 1,
		}}, false),
		Entry("min<max", []extensionsv1alpha1.WorkerPool{{
			Minimum: 0,
			Maximum: 1,
		}}, true),
	)

	DescribeTable("#GetDNSRecordType",
		func(address string, expected extensionsv1alpha1.DNSRecordType) {
			Expect(GetDNSRecordType(address)).To(Equal(expected))
		},

		Entry("valid IPv4 address", "1.2.3.4", extensionsv1alpha1.DNSRecordTypeA),
		Entry("valid IPv6 address", "2001:db8:f00::1", extensionsv1alpha1.DNSRecordTypeAAAA),
		Entry("anything else", "foo", extensionsv1alpha1.DNSRecordTypeCNAME),
	)

	DescribeTable("#GetDNSRecordTTL",
		func(ttl *int64, expected int64) {
			Expect(GetDNSRecordTTL(ttl)).To(Equal(expected))
		},

		Entry("nil value", nil, int64(120)),
		Entry("non-nil value", ptr.To[int64](300), int64(300)),
	)

	Describe("#DeterminePrimaryIPFamily", func() {
		It("should return IPv4 for empty ipFamilies", func() {
			Expect(DeterminePrimaryIPFamily(nil)).To(Equal(extensionsv1alpha1.IPFamilyIPv4))
			Expect(DeterminePrimaryIPFamily([]extensionsv1alpha1.IPFamily{})).To(Equal(extensionsv1alpha1.IPFamilyIPv4))
		})

		It("should return IPv4 if it's the first entry", func() {
			Expect(DeterminePrimaryIPFamily([]extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4})).To(Equal(extensionsv1alpha1.IPFamilyIPv4))
			Expect(DeterminePrimaryIPFamily([]extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv4, extensionsv1alpha1.IPFamilyIPv6})).To(Equal(extensionsv1alpha1.IPFamilyIPv4))
		})

		It("should return IPv6 if it's the first entry", func() {
			Expect(DeterminePrimaryIPFamily([]extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv6})).To(Equal(extensionsv1alpha1.IPFamilyIPv6))
			Expect(DeterminePrimaryIPFamily([]extensionsv1alpha1.IPFamily{extensionsv1alpha1.IPFamilyIPv6, extensionsv1alpha1.IPFamilyIPv4})).To(Equal(extensionsv1alpha1.IPFamilyIPv6))
		})
	})

	Describe("#FilePathsFrom", func() {
		It("should return the expected list", func() {
			file1 := extensionsv1alpha1.File{Path: "foo"}
			file2 := extensionsv1alpha1.File{Path: "bar"}

			Expect(FilePathsFrom([]extensionsv1alpha1.File{file1, file2})).To(ConsistOf("foo", "bar"))
		})
	})

	Describe("#GetMachineDeploymentClusterAutoscalerAnnotations", func() {
		It("should return nil when options passed is nil", func() {
			Expect(GetMachineDeploymentClusterAutoscalerAnnotations(nil)).To(BeNil())
		})

		It("should return empty map when an empty map is passed", func() {
			Expect(GetMachineDeploymentClusterAutoscalerAnnotations(ptr.To(extensionsv1alpha1.ClusterAutoscalerOptions{}))).To(Equal(map[string]string{}))
		})

		It("should return correctly populated map when all options are passed", func() {
			caOptions := &extensionsv1alpha1.ClusterAutoscalerOptions{
				ScaleDownUtilizationThreshold:    ptr.To("0.5"),
				ScaleDownGpuUtilizationThreshold: ptr.To("0.6"),
				ScaleDownUnneededTime:            ptr.To(metav1.Duration{Duration: time.Minute}),
				ScaleDownUnreadyTime:             ptr.To(metav1.Duration{Duration: 2 * time.Minute}),
				MaxNodeProvisionTime:             ptr.To(metav1.Duration{Duration: 3 * time.Minute}),
			}
			expectedValues := map[string]string{
				extensionsv1alpha1.ScaleDownUtilizationThresholdAnnotation:    "0.5",
				extensionsv1alpha1.ScaleDownGpuUtilizationThresholdAnnotation: "0.6",
				extensionsv1alpha1.ScaleDownUnneededTimeAnnotation:            "1m0s",
				extensionsv1alpha1.ScaleDownUnreadyTimeAnnotation:             "2m0s",
				extensionsv1alpha1.MaxNodeProvisionTimeAnnotation:             "3m0s",
			}
			Expect(GetMachineDeploymentClusterAutoscalerAnnotations(caOptions)).To(Equal(expectedValues))
		})

		It("should return correctly populated map when partial options are passed", func() {
			caOptions := &extensionsv1alpha1.ClusterAutoscalerOptions{
				ScaleDownGpuUtilizationThreshold: ptr.To("0.6"),
				ScaleDownUnneededTime:            ptr.To(metav1.Duration{Duration: time.Minute}),
			}
			expectedValues := map[string]string{
				extensionsv1alpha1.ScaleDownGpuUtilizationThresholdAnnotation: "0.6",
				extensionsv1alpha1.ScaleDownUnneededTimeAnnotation:            "1m0s",
			}
			Expect(GetMachineDeploymentClusterAutoscalerAnnotations(caOptions)).To(Equal(expectedValues))
		})
	})

	Describe("#HasContainerdConfiguration", func() {
		It("should return false when CRI config is nil", func() {
			Expect(HasContainerdConfiguration(nil)).To(BeFalse())
		})

		It("should return false when containerd is not configured", func() {
			Expect(HasContainerdConfiguration(&extensionsv1alpha1.CRIConfig{
				Name: "cri-o",
			})).To(BeFalse())
		})

		It("should return true when containerd is configured", func() {
			Expect(HasContainerdConfiguration(&extensionsv1alpha1.CRIConfig{
				Name:       "containerd",
				Containerd: &extensionsv1alpha1.ContainerdConfig{},
			})).To(BeTrue())
		})
	})
})

var _ = Describe("filecodec", func() {
	DescribeTable("#EncodeDecode",
		func(input extensionsv1alpha1.FileContentInline) {
			codeID, err := ParseFileCodecID(input.Encoding)
			Expect(err).NotTo(HaveOccurred())
			encoded, err := FileCodecForID(codeID).Encode([]byte(input.Data))
			Expect(err).NotTo(HaveOccurred())

			decoded, err := Decode(input.Encoding, encoded)
			Expect(err).NotTo(HaveOccurred())
			Expect(input.Data).To(Equal(string(decoded)))
		},

		Entry("plain", extensionsv1alpha1.FileContentInline{Encoding: "", Data: "plain data input"}),
		Entry("base64", extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: "base64 data input"}),
	)
})
