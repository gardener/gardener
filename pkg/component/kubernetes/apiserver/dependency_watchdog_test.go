// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserver_test

import (
	"time"

	proberapi "github.com/gardener/dependency-watchdog/api/prober"
	weederapi "github.com/gardener/dependency-watchdog/api/weeder"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
)

var _ = Describe("DependencyWatchdog", func() {
	Describe("#NewDependencyWatchdogWeederConfiguration", func() {
		It("should compute the correct configuration", func() {
			config, err := NewDependencyWatchdogWeederConfiguration()
			Expect(config).To(Equal(map[string]weederapi.DependantSelectors{
				"kube-apiserver": {
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
									Operator: metav1.LabelSelectorOpNotIn,
									Values:   []string{v1beta1constants.ETCDRoleMain, v1beta1constants.LabelAPIServer},
								},
							},
						},
					},
				},
			}))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#NewDependencyWatchdogProberConfiguration", func() {
		It("should compute the correct configuration", func() {
			config, err := NewDependencyWatchdogProberConfiguration()
			Expect(config).To(ConsistOf([]proberapi.DependentResourceInfo{
				{
					Ref: &autoscalingv1.CrossVersionObjectReference{
						Kind:       "Deployment",
						Name:       v1beta1constants.DeploymentNameKubeControllerManager,
						APIVersion: "apps/v1",
					},
					Optional: false,
					ScaleUpInfo: &proberapi.ScaleInfo{
						Level: 0,
					},
					ScaleDownInfo: &proberapi.ScaleInfo{
						Level: 1,
					},
				},
				{
					Ref: &autoscalingv1.CrossVersionObjectReference{
						Kind:       "Deployment",
						Name:       v1beta1constants.DeploymentNameMachineControllerManager,
						APIVersion: "apps/v1",
					},
					Optional: false,
					ScaleUpInfo: &proberapi.ScaleInfo{
						Level:        1,
						InitialDelay: &metav1.Duration{Duration: 30 * time.Second},
					},
					ScaleDownInfo: &proberapi.ScaleInfo{
						Level: 0,
					},
				},
				{
					Ref: &autoscalingv1.CrossVersionObjectReference{
						Kind:       "Deployment",
						Name:       "cluster-autoscaler",
						APIVersion: "apps/v1",
					},
					Optional: true,
					ScaleUpInfo: &proberapi.ScaleInfo{
						Level: 2,
					},
					ScaleDownInfo: &proberapi.ScaleInfo{
						Level: 0,
					},
				}}))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
