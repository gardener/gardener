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

package v1alpha1

import (
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/scheduler/apis/config/encoding"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
	landscaperv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	// defaultValidityCACertificates is 5 years
	defaultValidityCACertificates = metav1.Duration{Duration: time.Hour * 43800}
	// defaultValidityTLSCertificates is 1 year
	defaultValidityTLSCertificates = metav1.Duration{Duration: time.Hour * 8760}
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_Imports sets defaults for the configuration of the ControlPlane component.
func SetDefaults_Imports(obj *Imports) {
	if obj.Rbac != nil &&
		obj.Rbac.SeedAuthorizer != nil &&
		obj.Rbac.SeedAuthorizer.Enabled != nil &&
		*obj.Rbac.SeedAuthorizer.Enabled &&
		obj.GardenerAdmissionController != nil &&
		obj.GardenerAdmissionController.Enabled &&
		obj.GardenerAdmissionController.SeedRestriction == nil {
		obj.GardenerAdmissionController.SeedRestriction = &SeedRestriction{Enabled: true}
	}

	// GAPI defaults
	if obj.GardenerAPIServer == nil {
		obj.GardenerAPIServer = &GardenerAPIServer{}
	}

	if obj.GardenerAPIServer.ComponentConfiguration == nil {
		obj.GardenerAPIServer.ComponentConfiguration = &APIServerComponentConfiguration{}
	}

	if obj.GardenerAPIServer.ComponentConfiguration.CA == nil {
		obj.GardenerAPIServer.ComponentConfiguration.CA = &CA{}
	}

	if obj.GardenerAPIServer.ComponentConfiguration.CA.Validity == nil {
		obj.GardenerAPIServer.ComponentConfiguration.CA.Validity = &defaultValidityCACertificates
	}

	if obj.GardenerAPIServer.ComponentConfiguration.TLS == nil {
		obj.GardenerAPIServer.ComponentConfiguration.TLS = &TLSServer{}
	}

	if obj.GardenerAPIServer.ComponentConfiguration.TLS.Validity == nil {
		obj.GardenerAPIServer.ComponentConfiguration.TLS.Validity = &defaultValidityTLSCertificates
	}

	// GAC  defaults
	if obj.GardenerAdmissionController == nil {
		obj.GardenerAdmissionController = &GardenerAdmissionController{
			// this matches the default in the helm chart
			Enabled: true,
		}
	}

	if obj.GardenerAdmissionController.Enabled {
		if obj.GardenerAdmissionController.ComponentConfiguration == nil {
			obj.GardenerAdmissionController.ComponentConfiguration = &AdmissionControllerComponentConfiguration{}
		}

		if obj.GardenerAdmissionController.ComponentConfiguration.CA == nil {
			obj.GardenerAdmissionController.ComponentConfiguration.CA = &CA{}
		}

		if obj.GardenerAdmissionController.ComponentConfiguration.CA.Validity == nil {
			obj.GardenerAdmissionController.ComponentConfiguration.CA.Validity = &defaultValidityCACertificates
		}

		if obj.GardenerAdmissionController.ComponentConfiguration.TLS == nil {
			obj.GardenerAdmissionController.ComponentConfiguration.TLS = &TLSServer{}
		}

		if obj.GardenerAdmissionController.ComponentConfiguration.TLS.Validity == nil {
			obj.GardenerAdmissionController.ComponentConfiguration.TLS.Validity = &defaultValidityTLSCertificates
		}
	}

	if obj.GardenerAPIServer.ComponentConfiguration.Admission != nil &&
		obj.GardenerAPIServer.ComponentConfiguration.Admission.MutatingWebhook != nil &&
		obj.GardenerAPIServer.ComponentConfiguration.Admission.MutatingWebhook.TokenProjection != nil &&
		obj.GardenerAPIServer.ComponentConfiguration.Admission.MutatingWebhook.TokenProjection.Enabled &&
		obj.GardenerAPIServer.ComponentConfiguration.Admission.MutatingWebhook.Kubeconfig == nil {
		obj.GardenerAPIServer.ComponentConfiguration.Admission.MutatingWebhook.Kubeconfig = &landscaperv1alpha1.Target{
			Spec: landscaperv1alpha1.TargetSpec{
				Configuration: landscaperv1alpha1.AnyJSON{
					RawMessage: []byte(getVolumeProjectionKubeconfig("mutating")),
				},
			},
		}
	}

	if obj.GardenerAPIServer.ComponentConfiguration.Admission != nil &&
		obj.GardenerAPIServer.ComponentConfiguration.Admission.ValidatingWebhook != nil &&
		obj.GardenerAPIServer.ComponentConfiguration.Admission.ValidatingWebhook.TokenProjection != nil &&
		obj.GardenerAPIServer.ComponentConfiguration.Admission.ValidatingWebhook.TokenProjection.Enabled &&
		obj.GardenerAPIServer.ComponentConfiguration.Admission.ValidatingWebhook.Kubeconfig == nil {
		obj.GardenerAPIServer.ComponentConfiguration.Admission.ValidatingWebhook.Kubeconfig = &landscaperv1alpha1.Target{
			Spec: landscaperv1alpha1.TargetSpec{
				Configuration: landscaperv1alpha1.AnyJSON{
					RawMessage: []byte(getVolumeProjectionKubeconfig("validating")),
				},
			},
		}
	}

	// GCM defaults
	if obj.GardenerControllerManager == nil {
		obj.GardenerControllerManager = &GardenerControllerManager{}
	}

	if obj.GardenerControllerManager.ComponentConfiguration == nil {
		obj.GardenerControllerManager.ComponentConfiguration = &ControllerManagerComponentConfiguration{}
	}

	if obj.GardenerControllerManager.ComponentConfiguration.TLS == nil {
		obj.GardenerControllerManager.ComponentConfiguration.TLS = &TLSServer{}
	}

	if obj.GardenerControllerManager.ComponentConfiguration.TLS.Validity == nil {
		obj.GardenerControllerManager.ComponentConfiguration.TLS.Validity = &defaultValidityTLSCertificates
	}

	// Scheduler defaults
	if obj.GardenerScheduler == nil {
		obj.GardenerScheduler = &GardenerScheduler{}
	}

	if obj.GardenerScheduler.ComponentConfiguration == nil || obj.GardenerScheduler.ComponentConfiguration.Config.Object == nil && len(obj.GardenerScheduler.ComponentConfiguration.Config.Raw) == 0 {
		obj.GardenerScheduler.ComponentConfiguration = &SchedulerComponentConfiguration{
			Config: runtime.RawExtension{
				Object: &schedulerconfigv1alpha1.SchedulerConfiguration{},
			},
		}
	}

	schedulerConfig, err := encoding.DecodeSchedulerConfiguration(&obj.GardenerScheduler.ComponentConfiguration.Config, false)
	if err != nil {
		return
	}

	SetDefaultsSchedulerComponentConfiguration(schedulerConfig)

	obj.GardenerScheduler.ComponentConfiguration.Config = runtime.RawExtension{Object: schedulerConfig}
}

func getVolumeProjectionKubeconfig(name string) string {
	return fmt.Sprintf(`
---
apiVersion: v1
kind: Config
users:
- name: '*'
user:
  tokenFile: /var/run/secrets/admission-tokens/%s-webhook-token`, name)
}

// SetDefaultsSchedulerComponentConfiguration sets defaults for the Scheduler component configuration for the Landscaper imports
func SetDefaultsSchedulerComponentConfiguration(config *schedulerconfigv1alpha1.SchedulerConfiguration) {
	// setup the scheduler with the minimal distance strategy
	if config.Schedulers.Shoot == nil {
		config.Schedulers.Shoot = &schedulerconfigv1alpha1.ShootSchedulerConfiguration{
			Strategy: schedulerconfigv1alpha1.MinimalDistance,
		}
	}
}
