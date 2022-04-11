// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceDependants holds the service and the label selectors of the pods which has to be restarted when
// the service becomes ready and the pods are in CrashloopBackoff.
type ServiceDependants struct {
	Services  map[string]Service `json:"services"`
	Namespace string             `json:"namespace"`
}

// Service struct defines the dependent pods of a service.
type Service struct {
	Dependants []DependantPods `json:"dependantPods"`
}

// DependantPods struct captures the details needed to identify dependant pods.
type DependantPods struct {
	Name     string                `json:"name,omitempty"`
	Selector *metav1.LabelSelector `json:"selector"`
}
