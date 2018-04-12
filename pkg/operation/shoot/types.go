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

package shoot

import (
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

// Shoot is an object containing information about a Shoot cluster.
type Shoot struct {
	Info                        *gardenv1beta1.Shoot
	Secret                      *corev1.Secret
	CloudProfile                *gardenv1beta1.CloudProfile
	CloudProvider               gardenv1beta1.CloudProvider
	SeedNamespace               string
	InternalClusterDomain       string
	ExternalClusterDomain       *string
	KubernetesMajorMinorVersion string
	Hibernated                  bool
}
