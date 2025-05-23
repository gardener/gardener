// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/autoscaling/vpa"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	kubecontrollermanager "github.com/gardener/gardener/pkg/component/kubernetes/controllermanager"
	"github.com/gardener/gardener/pkg/component/networking/nginxingress"
	"github.com/gardener/gardener/pkg/component/observability/logging/vali"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/kubestatemetrics"
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
	vpa.CentralLoggingConfiguration,
	vali.CentralLoggingConfiguration,
	kubestatemetrics.CentralLoggingConfiguration,
	// virtual garden control plane components
	etcd.CentralLoggingConfiguration,
	kubeapiserver.CentralLoggingConfiguration,
	kubecontrollermanager.CentralLoggingConfiguration,
}
