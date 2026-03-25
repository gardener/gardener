// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1_test

import (
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
)

var _ = Describe("JSON", func() {
	Describe("TokenRequestorWorkloadIdentityControllerConfiguration", func() {
		DescribeTable("UnmarshalJSON",
			func(input string, expected TokenRequestorWorkloadIdentityControllerConfiguration) {
				var cfg TokenRequestorWorkloadIdentityControllerConfiguration
				Expect(json.Unmarshal([]byte(input), &cfg)).To(Succeed())
				Expect(cfg).To(Equal(expected))
			},
			Entry("new format with string duration",
				`{"concurrentSyncs":10,"tokenExpirationDuration":"6h"}`,
				TokenRequestorWorkloadIdentityControllerConfiguration{
					ConcurrentSyncs:         ptr.To(10),
					TokenExpirationDuration: &metav1.Duration{Duration: 6 * time.Hour},
				},
			),
			Entry("old format with numeric duration (nanoseconds)",
				`{"concurrentSyncs":10,"tokenExpirationDuration":21600000000000}`,
				TokenRequestorWorkloadIdentityControllerConfiguration{
					ConcurrentSyncs:         ptr.To(10),
					TokenExpirationDuration: &metav1.Duration{Duration: 6 * time.Hour},
				},
			),
			Entry("empty object",
				`{}`,
				TokenRequestorWorkloadIdentityControllerConfiguration{},
			),
			Entry("only concurrentSyncs",
				`{"concurrentSyncs":5}`,
				TokenRequestorWorkloadIdentityControllerConfiguration{
					ConcurrentSyncs: ptr.To(5),
				},
			),
			Entry("only new format duration",
				`{"tokenExpirationDuration":"30m"}`,
				TokenRequestorWorkloadIdentityControllerConfiguration{
					TokenExpirationDuration: &metav1.Duration{Duration: 30 * time.Minute},
				},
			),
			Entry("only old format duration",
				`{"tokenExpirationDuration":1800000000000}`,
				TokenRequestorWorkloadIdentityControllerConfiguration{
					TokenExpirationDuration: &metav1.Duration{Duration: 30 * time.Minute},
				},
			),
		)
	})
})
