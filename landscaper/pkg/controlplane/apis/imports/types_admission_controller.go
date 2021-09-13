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

package imports

// GardenerAdmissionController contains the configuration of the Gardener Admission Controller
type GardenerAdmissionController struct {
	// Enabled configures whether the Gardener Admission Controller should be deployed
	Enabled bool
	// SeedRestriction configures the SeedRestriction admission plugin
	SeedRestriction *SeedRestriction
	// DeploymentConfiguration contains optional configurations for
	// the deployment of the Gardener Admission Controller
	DeploymentConfiguration *CommonDeploymentConfiguration
	// ComponentConfiguration contains the component configuration of the Gardener API Server
	ComponentConfiguration *AdmissionControllerComponentConfiguration
}

// AdmissionControllerComponentConfiguration contains the component configuration of the Gardener Admission Controller
type AdmissionControllerComponentConfiguration struct {
	// CABundle is a PEM encoded CA bundle which will be used by the Gardener API server
	// to validate the TLS serving certificate of the Gardener Admission Webhook server served
	// by the Gardener Admission Controller.
	// It is put into the MutatingWebhookConfiguration and ValidatingWebhookConfiguration
	// resources when registering the Webhooks.
	// If left empty, generates a new CA or reuses the CA of an existing Admission controller deployment.
	CABundle *string
	// TLS configures the TLS serving certificate of the Gardener Admission Controller webhooks.
	// The certificate has to be signed by the provided CA bundle.
	// If left empty, generates certificates signed by the provided CA bundle.
	TLS *TLSServer
	// Component specifies values for the Gardener Admission Controller component configuration
	// Please see example/20-componentconfig-gardener-admission-controller.yaml for what
	// can be configured here
	*Configuration
}

// SeedRestriction configures the SeedRestriction admission plugin
type SeedRestriction struct {
	// Enabled configures whether the SeedRestriction admission plugin should be enabled.
	// Sets up the ValidatingWebhookConfiguration pointing to the webhook server in the Gardener Admission Controller
	// serving the SeedRestriction webhook
	// If the Seed Authorizer is enabled already, the Seed Restriction Plugin will be enabled per default.
	// Default: false
	Enabled bool
}
