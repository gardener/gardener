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

package logging

import (
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/docker"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/component/hvpa"
	"github.com/gardener/gardener/pkg/component/kubeapiserver"
	"github.com/gardener/gardener/pkg/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/component/logging/vali"
	"github.com/gardener/gardener/pkg/component/nginxingress"
	"github.com/gardener/gardener/pkg/component/resourcemanager"
	"github.com/gardener/gardener/pkg/component/vpa"
)

// GardenCentralLoggingConfigurations is a list of central logging configuration for components running in the garden
// cluster.
var GardenCentralLoggingConfigurations = []component.CentralLoggingConfiguration{
	// Ensure kubelet/container runtime logs get parsed and forwarded to vali
	kubelet.CentralLoggingConfiguration,
	docker.CentralLoggingConfiguration,
	containerd.CentralLoggingConfiguration,
	// garden system components
	resourcemanager.CentralLoggingConfiguration,
	nginxingress.CentralLoggingConfiguration,
	hvpa.CentralLoggingConfiguration,
	vpa.CentralLoggingConfiguration,
	vali.CentralLoggingConfiguration,
	kubestatemetrics.CentralLoggingConfiguration,
	// virtual garden control plane components
	etcd.CentralLoggingConfiguration,
	kubeapiserver.CentralLoggingConfiguration,
	kubecontrollermanager.CentralLoggingConfiguration,
}
