// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
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
