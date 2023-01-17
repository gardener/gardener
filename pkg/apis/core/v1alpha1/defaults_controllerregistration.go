// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// SetDefaults_ControllerResource sets default values for ControllerResource objects.
func SetDefaults_ControllerResource(obj *ControllerResource) {
	if obj.Primary == nil {
		obj.Primary = pointer.Bool(true)
	}

	if obj.Kind == "Extension" {
		if obj.GloballyEnabled == nil {
			obj.GloballyEnabled = pointer.Bool(false)
		}

		if obj.ReconcileTimeout == nil {
			obj.ReconcileTimeout = &metav1.Duration{Duration: time.Minute * 3}
		}

		if obj.Lifecycle == nil {
			obj.Lifecycle = &ControllerResourceLifecycle{}
		}
	}
}

// SetDefaults_ControllerResourceLifecycle sets default values for ControllerResourceLifecycle objects.
func SetDefaults_ControllerResourceLifecycle(obj *ControllerResourceLifecycle) {
	if obj.Reconcile == nil {
		afterKubeAPIServer := AfterKubeAPIServer
		obj.Reconcile = &afterKubeAPIServer
	}
	if obj.Delete == nil {
		beforeKubeAPIServer := BeforeKubeAPIServer
		obj.Delete = &beforeKubeAPIServer
	}
	if obj.Migrate == nil {
		beforeKubeAPIServer := BeforeKubeAPIServer
		obj.Migrate = &beforeKubeAPIServer
	}
}

// SetDefaults_ControllerRegistrationDeployment sets default values for ControllerRegistrationDeployment objects.
func SetDefaults_ControllerRegistrationDeployment(obj *ControllerRegistrationDeployment) {
	p := ControllerDeploymentPolicyOnDemand
	if obj.Policy == nil {
		obj.Policy = &p
	}
}
