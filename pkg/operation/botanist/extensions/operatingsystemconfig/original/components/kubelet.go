// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package components

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigurableKubeletCLIFlags is the set of configurable kubelet command line parameters.
type ConfigurableKubeletCLIFlags struct {
	ImagePullProgressDeadline *metav1.Duration
}

// ConfigurableKubeletConfigParameters is the set of configurable kubelet config parameters.
type ConfigurableKubeletConfigParameters struct {
	CpuCFSQuota                      *bool
	CpuManagerPolicy                 *string
	EvictionHard                     map[string]string
	EvictionMinimumReclaim           map[string]string
	EvictionSoft                     map[string]string
	EvictionSoftGracePeriod          map[string]string
	EvictionPressureTransitionPeriod *metav1.Duration
	EvictionMaxPodGracePeriod        *int32
	FailSwapOn                       *bool
	FeatureGates                     map[string]bool
	KubeReserved                     map[string]string
	MaxPods                          *int32
	PodPidsLimit                     *int64
	SystemReserved                   map[string]string
}
