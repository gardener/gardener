// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// SetDefaults_ControllerResource sets default values for ControllerResource objects.
func SetDefaults_ControllerResource(obj *ControllerResource) {
	if obj.Primary == nil {
		obj.Primary = ptr.To(true)
	}

	if obj.Kind == "Extension" {
		if obj.GloballyEnabled == nil {
			obj.GloballyEnabled = ptr.To(false)
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
