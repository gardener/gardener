// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1/helper"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
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
})
