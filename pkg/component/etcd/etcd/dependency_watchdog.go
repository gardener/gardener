// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	weederapi "github.com/gardener/dependency-watchdog/api/weeder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
)

// NewDependencyWatchdogWeederConfiguration returns the configuration for the dependency watchdog ensuring that its dependant
// pods are restarted as soon as it recovers from a crash loop.
func NewDependencyWatchdogWeederConfiguration(role string) (map[string]weederapi.DependantSelectors, error) {
	return map[string]weederapi.DependantSelectors{
		etcdconstants.ServiceName(role): {
			PodSelectors: []*metav1.LabelSelector{
				{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      v1beta1constants.GardenRole,
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{v1beta1constants.GardenRoleControlPlane},
						},
						{
							Key:      v1beta1constants.LabelRole,
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{v1beta1constants.LabelAPIServer},
						},
					},
				},
			},
		},
	}, nil
}
