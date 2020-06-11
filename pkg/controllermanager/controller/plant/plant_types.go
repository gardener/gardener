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

package plant

import (
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"

	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/record"
)

type defaultPlantControl struct {
	clientMap     clientmap.ClientMap
	secretsLister kubecorev1listers.SecretLister
	recorder      record.EventRecorder
	config        *config.ControllerManagerConfiguration
}

// StatusCloudInfo contains the cloud info for the plant status
type StatusCloudInfo struct {
	CloudType  string
	Region     string
	K8sVersion string
}
