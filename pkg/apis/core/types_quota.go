// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package core

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Quota struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec defines the Quota constraints.
	Spec QuotaSpec
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuotaList is a collection of Quotas.
type QuotaList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta
	// Items is the list of Quotas.
	Items []Quota
}

// QuotaSpec is the specification of a Quota.
type QuotaSpec struct {
	// ClusterLifetimeDays is the lifetime of a Shoot cluster in days before it will be terminated automatically.
	ClusterLifetimeDays *int32
	// Metrics is a list of resources which will be put under constraints.
	Metrics corev1.ResourceList
	// Scope is the scope of the Quota object, either 'project' or 'secret'.
	Scope corev1.ObjectReference
}

const (
	// QuotaMetricCPU is the constraint for the amount of CPUs
	QuotaMetricCPU corev1.ResourceName = corev1.ResourceCPU
	// QuotaMetricGPU is the constraint for the amount of GPUs (e.g. from Nvidia)
	QuotaMetricGPU corev1.ResourceName = "gpu"
	// QuotaMetricMemory is the constraint for the amount of memory
	QuotaMetricMemory corev1.ResourceName = corev1.ResourceMemory
	// QuotaMetricStorageStandard is the constraint for the size of a standard disk
	QuotaMetricStorageStandard corev1.ResourceName = corev1.ResourceStorage + ".standard"
	// QuotaMetricStoragePremium is the constraint for the size of a premium disk (e.g. SSD)
	QuotaMetricStoragePremium corev1.ResourceName = corev1.ResourceStorage + ".premium"
	// QuotaMetricLoadbalancer is the constraint for the amount of loadbalancers
	QuotaMetricLoadbalancer corev1.ResourceName = "loadbalancer"
)
