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

package terraformer

import (
	"path/filepath"

	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
)

// Terraformer is a struct containing configuration parameters for the Terraform script it acts on.
// * Operation is a reference to an operation object.
// * Purpose is a one-word description depicting what the Terraformer does (e.g. 'infrastructure').
// * Namespace is the namespace in which the Terraformer will act (usually the Shoot namespace).
// * ConfigName is the name of the ConfigMap containing the main Terraform file ('main.tf').
// * VariablesName is the name of the Secret containing the Terraform variables ('terraform.tfvars').
// * StateName is the name of the ConfigMap containing the Terraform state ('terraform.tfstate').
// * PodName is the name of the Pod which will validate the Terraform file.
// * JobName is the name of the Job which will execute the Terraform file.
// * VariablesEnvironment is a map of environment variables which will be injected in the resulting
//   Terraform job/pod. These variables should contain Terraform variables (i.e., must be prefixed
//   with TF_VAR_).
// * ConfigurationDefined indicates whether the required configuration ConfigMaps/Secrets have been
//   successfully defined.
type Terraformer struct {
	*operation.Operation
	Purpose              string
	Namespace            string
	ConfigName           string
	VariablesName        string
	StateName            string
	PodName              string
	JobName              string
	VariablesEnvironment []map[string]interface{}
	ConfigurationDefined bool
}

var chartPath = filepath.Join(common.ChartPath, "seed-terraformer", "charts")
