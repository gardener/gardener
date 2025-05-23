// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Shoot", func() {
	trueVar := true
	falseVar := false
	lastOperationCreate := gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeCreate}
	lastOperationReconcile := gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeReconcile}

	cidr := "10.250.0.0/19"
	cidrV6 := "2001:db8::/64"

	DescribeTable("#Get*Network",
		func(cluster *Cluster, functionUnderTest func(*Cluster) []string, cidrs []string) {
			Expect(functionUnderTest(cluster)).To(Equal(cidrs))
		},

		Entry("pod cidr is given", &Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						Pods: &cidr,
					},
				},
			},
		}, GetPodNetwork, []string{cidr}),
		Entry("pod cidr is given + shoot status", &Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						Pods: &cidr,
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					Networking: &gardencorev1beta1.NetworkingStatus{
						Pods: []string{cidr},
					},
				},
			},
		}, GetPodNetwork, []string{cidr}),
		Entry("pod cidr is given + shoot status, but different cidrs", &Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						Pods: &cidr,
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					Networking: &gardencorev1beta1.NetworkingStatus{
						Pods: []string{cidrV6},
					},
				},
			},
		}, GetPodNetwork, []string{cidr, cidrV6}),
		Entry("dual-stack pod cidr in shoot status", &Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					Networking: &gardencorev1beta1.NetworkingStatus{
						Pods: []string{cidr, cidrV6},
					},
				},
			},
		}, GetPodNetwork, []string{cidr, cidrV6}),
		Entry("service cidr is given", &Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						Services: &cidr,
					},
				},
			},
		}, GetServiceNetwork, []string{cidr}),
		Entry("service cidr is given + shoot status", &Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						Services: &cidr,
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					Networking: &gardencorev1beta1.NetworkingStatus{
						Services: []string{cidr},
					},
				},
			},
		}, GetServiceNetwork, []string{cidr}),
		Entry("service cidr is given + shoot status, but different cidrs", &Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{
						Services: &cidr,
					},
				},
				Status: gardencorev1beta1.ShootStatus{
					Networking: &gardencorev1beta1.NetworkingStatus{
						Services: []string{cidrV6},
					},
				},
			},
		}, GetServiceNetwork, []string{cidr, cidrV6}),
		Entry("dual-stack service cidr in shoot status", &Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					Networking: &gardencorev1beta1.NetworkingStatus{
						Services: []string{cidr, cidrV6},
					},
				},
			},
		}, GetServiceNetwork, []string{cidr, cidrV6}),
	)

	DescribeTable("#IsHibernationEnabled",
		func(hibernation *gardencorev1beta1.Hibernation, expectation bool) {
			cluster := &Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Hibernation: hibernation,
					},
				},
			}

			Expect(IsHibernationEnabled(cluster)).To(Equal(expectation))
		},

		Entry("hibernation is nil", nil, false),
		Entry("hibernation is not enabled", &gardencorev1beta1.Hibernation{Enabled: &falseVar}, false),
		Entry("hibernation is enabled", &gardencorev1beta1.Hibernation{Enabled: &trueVar}, true),
	)

	DescribeTable("#IsHibernated",
		func(hibernation *gardencorev1beta1.Hibernation, isHibernated bool, expectation bool) {
			cluster := &Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Hibernation: hibernation,
					},
					Status: gardencorev1beta1.ShootStatus{IsHibernated: isHibernated},
				},
			}
			Expect(IsHibernated(cluster)).To(Equal(expectation))
		},
		Entry("spec hibernation is nil", nil, false, false),
		Entry("spec hibernation is not enabled", &gardencorev1beta1.Hibernation{Enabled: &falseVar}, false, false),
		Entry("hibernation is enabled, status is not hibernated", &gardencorev1beta1.Hibernation{Enabled: &trueVar}, false, false),
		Entry("hibernation is enabled, status is hibernated", &gardencorev1beta1.Hibernation{Enabled: &trueVar}, true, true),
	)

	DescribeTable("#IsHibernatingOrWakingUp",
		func(hibernation *gardencorev1beta1.Hibernation, isHibernated bool, expectation bool) {
			cluster := &Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Hibernation: hibernation,
					},
					Status: gardencorev1beta1.ShootStatus{IsHibernated: isHibernated},
				},
			}
			Expect(IsHibernatingOrWakingUp(cluster)).To(Equal(expectation))
		},
		Entry("spec hibernation is nil", nil, false, false),
		Entry("spec hibernation is not enabled and it is not hibernated", &gardencorev1beta1.Hibernation{Enabled: &falseVar}, false, false),
		Entry("hibernation is enabled, status is not hibernated", &gardencorev1beta1.Hibernation{Enabled: &trueVar}, false, true),
		Entry("hibernation is not enabled, status is hibernated", &gardencorev1beta1.Hibernation{Enabled: &falseVar}, true, true),
		Entry("hibernation is enabled, status is hibernated", &gardencorev1beta1.Hibernation{Enabled: &trueVar}, true, false),
	)

	DescribeTable("#IsCreationInProgress",
		func(lastOperation *gardencorev1beta1.LastOperation, expectation bool) {
			cluster := &Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						LastOperation: lastOperation,
					},
				},
			}
			Expect(IsCreationInProcess(cluster)).To(Equal(expectation))
		},
		Entry("last operation is nil", nil, true),
		Entry("last operation is create", &lastOperationCreate, true),
		Entry("last operation is reconcile", &lastOperationReconcile, false),
	)

	var (
		dnsDomain            = "dnsdomain"
		dnsProviderType      = "type"
		dnsProviderUnmanaged = "unmanaged"
	)

	DescribeTable("#IsUnmanagedDNSProvider",
		func(dns *gardencorev1beta1.DNS, expectation bool) {
			cluster := &Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						DNS: dns,
					},
				},
			}

			Expect(IsUnmanagedDNSProvider(cluster)).To(Equal(expectation))
		},

		Entry("dns is nil", nil, true),
		Entry("dns domain is set", &gardencorev1beta1.DNS{
			Domain: &dnsDomain,
		}, false),
		Entry("dns domain is not set and provider is not given", &gardencorev1beta1.DNS{
			Providers: []gardencorev1beta1.DNSProvider{},
		}, false),
		Entry("dns domain is not set and provider is given but type is not unmanaged", &gardencorev1beta1.DNS{
			Providers: []gardencorev1beta1.DNSProvider{{
				Type: &dnsProviderType,
			}},
		}, false),
		Entry("dns domain is not set and provider is given and type is unmanaged", &gardencorev1beta1.DNS{
			Providers: []gardencorev1beta1.DNSProvider{{
				Type: &dnsProviderUnmanaged,
			}},
		}, true),
	)

	DescribeTable("#GetReplicas",
		func(hibernation *gardencorev1beta1.Hibernation, wokenUp, expectation int) {
			cluster := &Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Spec: gardencorev1beta1.ShootSpec{
						Hibernation: hibernation,
					},
				},
			}

			Expect(GetReplicas(cluster, wokenUp)).To(Equal(expectation))
		},

		Entry("hibernation is not enabled", nil, 3, 3),
		Entry("hibernation is enabled", &gardencorev1beta1.Hibernation{Enabled: &trueVar}, 1, 0),
	)

	DescribeTable("#IsFailed",
		func(lastOperation *gardencorev1beta1.LastOperation, expectedToBeFailed bool) {
			cluster := &Cluster{
				Shoot: &gardencorev1beta1.Shoot{
					Status: gardencorev1beta1.ShootStatus{
						LastOperation: lastOperation,
					},
				},
			}

			Expect(IsFailed(cluster)).To(Equal(expectedToBeFailed))
		},

		Entry("cluster is failed", &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateFailed}, true),
		Entry("cluster is not failed", &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateError}, false),
		Entry("cluster is not failed", nil, false),
	)

	DescribeTable("#IsShootFailed",
		func(lastOperation *gardencorev1beta1.LastOperation, expectedToBeFailed bool) {
			shoot := &gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: lastOperation,
				},
			}
			Expect(IsShootFailed(shoot)).To(Equal(expectedToBeFailed))
		},

		Entry("cluster is failed", &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateFailed}, true),
		Entry("cluster is not failed", &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateError}, false),
		Entry("cluster is not failed", nil, false),
	)
})
