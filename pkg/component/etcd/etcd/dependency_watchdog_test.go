// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd_test

import (
	weederapi "github.com/gardener/dependency-watchdog/api/weeder"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
)

var _ = Describe("DependencyWatchdog", func() {
	Describe("#NewDependencyWatchdogWeederConfiguration", func() {
		It("should compute the correct configuration", func() {
			config, err := etcd.NewDependencyWatchdogWeederConfiguration(testRole)
			Expect(config).To(Equal(map[string]weederapi.DependantSelectors{
				"etcd-" + testRole + "-client": {
					PodSelectors: []*metav1.LabelSelector{
						{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      v1beta1constants.GardenRole,
									Operator: "In",
									Values:   []string{v1beta1constants.GardenRoleControlPlane},
								},
								{
									Key:      v1beta1constants.LabelRole,
									Operator: "In",
									Values:   []string{v1beta1constants.LabelAPIServer},
								},
							},
						},
					},
				},
			}))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
