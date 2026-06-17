// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	autoscalingv1 "k8s.io/api/autoscaling/v1"

	. "github.com/gardener/gardener/pkg/api/core/v1/helper"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
)

var _ = Describe("Helper", func() {
	DescribeTable("#ResourceReferencesEqual",
		func(oldResources, newResources []gardencorev1.NamedResourceReference, matcher gomegatypes.GomegaMatcher) {
			Expect(ResourceReferencesEqual(oldResources, newResources)).To(matcher)
		},

		Entry("both nil", nil, nil, BeTrue()),
		Entry("old empty, new w/o secrets", []gardencorev1.NamedResourceReference{}, []gardencorev1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{Name: "foo"}}}, BeTrue()),
		Entry("old empty, new w/ secrets", []gardencorev1.NamedResourceReference{}, []gardencorev1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, BeFalse()),
		Entry("old empty, new w/ configMap", []gardencorev1.NamedResourceReference{}, []gardencorev1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "foo"}}}, BeFalse()),
		Entry("old empty, new w/ workloadIdentity", []gardencorev1.NamedResourceReference{}, []gardencorev1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "security.gardener.cloud/v1alpha1", Kind: "WorkloadIdentity", Name: "foo"}}}, BeFalse()),
		Entry("old w/o secrets, new empty", []gardencorev1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{Name: "foo"}}}, []gardencorev1.NamedResourceReference{}, BeTrue()),
		Entry("old w/ secrets, new empty", []gardencorev1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, []gardencorev1.NamedResourceReference{}, BeFalse()),
		Entry("difference", []gardencorev1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, []gardencorev1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "bar"}}}, BeFalse()),
		Entry("difference because no secret", []gardencorev1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, []gardencorev1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "bar"}}}, BeFalse()),
		Entry("difference because new is configMap with same name", []gardencorev1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, []gardencorev1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "ConfigMap", Name: "foo"}}}, BeFalse()),
		Entry("difference because new is workloadIdentity with other name", []gardencorev1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "security.gardener.cloud/v1alpha1", Kind: "WorkloadIdentity", Name: "foo"}}}, []gardencorev1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "security.gardener.cloud/v1alpha1", Kind: "WorkloadIdentity", Name: "bar"}}}, BeFalse()),
		Entry("equality", []gardencorev1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, []gardencorev1.NamedResourceReference{{ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo"}}}, BeTrue()),
	)
})
