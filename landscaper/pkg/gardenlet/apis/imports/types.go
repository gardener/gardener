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

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	landscaperv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"

	"github.com/gardener/gardener/pkg/apis/seedmanagement"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Imports defines the landscaper import for the Gardenlet.
type Imports struct {
	metav1.TypeMeta
	// SeedCluster contains the kubeconfig for the cluster
	// - into which the Gardenlet is deployed by the landscaper
	// - that is targeted as the Seed cluster by the Gardenlet via the default in-cluster mounted service account token
	// Hence, the Gardenlet is always deployed into the Seed cluster itself.
	// Deploying the Gardenlet outside of the Seed cluster is not supported (e.g., landscaper deploys Gardenlet into
	// cluster A and the Gardenlet is configured via field .seedClientConnection.kubeconfig to target cluster B as Seed)
	SeedCluster landscaperv1alpha1.Target
	// GardenCluster is the landscaper target containing the kubeconfig for the
	// Garden cluster (sometimes referred to as "virtual garden" - with Gardener API groups!)
	GardenCluster landscaperv1alpha1.Target
	// SeedBackupCredentials contains the credentials for an optional backup provider for the Seed cluster registered by the Gardenlet
	// required when the Seed is configured for Backup (configured in the Gardenlet component configuration in the field seedConfig.spec.backup).
	// Before deploying the Gardenlet, the landscaper deploys a secret containing the specified credentials into the Garden cluster
	SeedBackupCredentials *json.RawMessage
	// DeploymentConfiguration configures the Kubernetes deployment of the Gardenlet
	DeploymentConfiguration *seedmanagement.GardenletDeployment
	// ComponentConfiguration specifies values for the Gardenlet component configuration
	// This results in the configuration file loaded at runtime by the deployed Gardenlet
	ComponentConfiguration runtime.Object
	// ImageVectorOverwrite contains an optional image vector override.
	ImageVectorOverwrite *json.RawMessage
	// ComponentImageVectorOverwrites contains an optional image vector override for components deployed by the gardenlet.
	ComponentImageVectorOverwrites *json.RawMessage
}
