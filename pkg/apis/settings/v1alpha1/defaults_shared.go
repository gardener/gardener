// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// SetDefaults_OpenIDConnectPresetSpec sets default values for OpenIDConnectPresetSpec objects.
func SetDefaults_OpenIDConnectPresetSpec(obj *OpenIDConnectPresetSpec) {
	if obj.ShootSelector == nil {
		obj.ShootSelector = &metav1.LabelSelector{}
	}
}

// SetDefaults_KubeAPIServerOpenIDConnect sets default values for KubeAPIServerOpenIDConnect objects.
func SetDefaults_KubeAPIServerOpenIDConnect(obj *KubeAPIServerOpenIDConnect) {
	if len(obj.SigningAlgs) == 0 {
		obj.SigningAlgs = []string{DefaultSignAlg}
	}

	if obj.UsernameClaim == nil {
		obj.UsernameClaim = ptr.To(DefaultUsernameClaim)
	}
}
