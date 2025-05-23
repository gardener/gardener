// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package fluentcustomresources_test

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2"
	fluentbitv1alpha2filter "github.com/fluent/fluent-operator/v3/apis/fluentbit/v1alpha2/plugins/filter"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/component/observability/logging/fluentcustomresources"
)

var _ = Describe("Logging", func() {
	Describe("#GetClusterFilters", func() {
		var (
			configName = "some-name"
			labels     = map[string]string{"some-key": "some-value"}
		)

		It("should return the expected ClusterFilter custom resources", func() {
			fluentBitClusterFilters := GetClusterFilters(configName, labels)

			Expect(fluentBitClusterFilters).To(Equal(
				[]*fluentbitv1alpha2.ClusterFilter{
					{
						ObjectMeta: metav1.ObjectMeta{
							// This filter will be the second one of fluent-bit because the operator orders them by name
							Name:   "02-containerd",
							Labels: labels,
						},
						Spec: fluentbitv1alpha2.FilterSpec{
							Match: "kubernetes.*",
							FilterItems: []fluentbitv1alpha2.FilterItem{
								{
									Parser: &fluentbitv1alpha2filter.Parser{
										KeyName:     "log",
										Parser:      "containerd-parser",
										ReserveData: ptr.To(true),
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							// This filter will be the third one of fluent-bit because the operator orders them by name
							Name:   "03-add-tag-to-record",
							Labels: labels,
						},
						Spec: fluentbitv1alpha2.FilterSpec{
							Match: "kubernetes.*",
							FilterItems: []fluentbitv1alpha2.FilterItem{
								{
									Lua: &fluentbitv1alpha2filter.Lua{
										Script: corev1.ConfigMapKeySelector{
											Key: "add_tag_to_record.lua",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: configName,
											},
										},
										Call: "add_tag_to_record",
									},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							// This filter will be the last one of fluent-bit because the operator orders them by name
							Name:   "zz-modify-severity",
							Labels: labels,
						},
						Spec: fluentbitv1alpha2.FilterSpec{
							Match: "kubernetes.*",
							FilterItems: []fluentbitv1alpha2.FilterItem{
								{
									Lua: &fluentbitv1alpha2filter.Lua{
										Script: corev1.ConfigMapKeySelector{
											Key: "modify_severity.lua",
											LocalObjectReference: corev1.LocalObjectReference{
												Name: configName,
											},
										},
										Call: "cb_modify",
									},
								},
							},
						},
					},
				}))
		})
	})
})
