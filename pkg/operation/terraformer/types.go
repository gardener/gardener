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

	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/sirupsen/logrus"
)

// Terraformer is a struct containing configuration parameters for the Terraform script it acts on.
// * purpose is a one-word description depicting what the Terraformer does (e.g. 'infrastructure').
// * namespace is the namespace in which the Terraformer will act.
// * image is the Docker image name of the Terraformer image.
// * configName is the name of the ConfigMap containing the main Terraform file ('main.tf').
// * variablesName is the name of the Secret containing the Terraform variables ('terraform.tfvars').
// * stateName is the name of the ConfigMap containing the Terraform state ('terraform.tfstate').
// * podName is the name of the Pod which will validate the Terraform file.
// * jobName is the name of the Job which will execute the Terraform file.
// * variablesEnvironment is a map of environment variables which will be injected in the resulting
//   Terraform job/pod. These variables should contain Terraform variables (i.e., must be prefixed
//   with TF_VAR_).
// * configurationDefined indicates whether the required configuration ConfigMaps/Secrets have been
//   successfully defined.
type Terraformer struct {
	logger        *logrus.Entry
	k8sClient     kubernetes.Client
	chartRenderer chartrenderer.ChartRenderer

	purpose   string
	namespace string
	image     string

	configName           string
	variablesName        string
	stateName            string
	podName              string
	jobName              string
	variablesEnvironment []map[string]interface{}
	configurationDefined bool
}

var chartPath = filepath.Join(common.ChartPath, "seed-terraformer", "charts")
