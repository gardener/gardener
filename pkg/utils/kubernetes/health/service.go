// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// CheckService checks whether the given service is healthy.
// A Service is considered unhealthy if it is of type `LoadBalancer` but doesn't have an ingress element in its status.
func CheckService(service *corev1.Service) error {
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return nil
	}
	if len(service.Status.LoadBalancer.Ingress) > 0 {
		return nil
	}
	return fmt.Errorf("service is missing ingress status")
}
