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

package values

import (
	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RuntimeChartValuesHelper provides methods computing the values to be used when applying the control plane runtime chart
type RuntimeChartValuesHelper interface {
	// GetRuntimeChartValues computes the values to be used when applying the control plane runtime chart.
	GetRuntimeChartValues() (map[string]interface{}, error)
}

// runtimeValuesHelper is a concrete implementation of RuntimeChartValuesHelper
// Contains all values that are needed to render the control plane runtime chart
type runtimeValuesHelper struct {
	// RuntimeClient targets the runtime cluster
	RuntimeClient client.Client
	// VirtualGarden defines if the application chart is installed into a virtual Garden cluster
	// This causes the service 'gardener-apiserver' in the runtime cluster to contain a clusterIP that is referenced by the
	// endpoint 'gardener-apiserver' in the virtual garden cluster.
	VirtualGarden bool // .Values.global.deployment.virtualGarden.enabled
	// VirtualGardenKubeconfigGardenerAPIServer is the generated Kubeconfig for the Gardener API Server
	// Only set when deploying using a virtual garden
	VirtualGardenKubeconfigGardenerAPIServer *string
	// VirtualGardenKubeconfigGardenerControllerManager is the generated Kubeconfig for the Gardener Controller Manager
	// Only set when deploying using a virtual garden
	VirtualGardenKubeconfigGardenerControllerManager *string
	// VirtualGardenKubeconfigGardenerScheduler is the generated Kubeconfig for the Gardener Scheduler
	// Only set when deploying using a virtual garden
	VirtualGardenKubeconfigGardenerScheduler *string
	// VirtualGardenKubeconfigGardenerAdmissionController is the generated Kubeconfig for the Gardener Admission Controller
	// Only set when deploying using a virtual garden
	VirtualGardenKubeconfigGardenerAdmissionController *string
	// AdmissionControllerConfig is the configuration of the Gardener Admission Controller
	// Needed for the runtime chart to deploy the config map containing the component configuration of the Gardener Admission Controller
	AdmissionControllerConfig *admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration
	// ControllerManagerConfig is the configuration of the Gardener Controller Manager
	// Needed for the runtime chart to deploy the config map containing the component configuration of the Gardener Controller Manager
	ControllerManagerConfig *controllermanagerconfigv1alpha1.ControllerManagerConfiguration
	// SchedulerConfig is the configuration of the Gardener Scheduler
	// Needed for the runtime chart to deploy the config map containing the component configuration of the Gardener Scheduler
	SchedulerConfig *schedulerconfigv1alpha1.SchedulerConfiguration
}

// NewRuntimeChartValuesHelper creates a new RuntimeChartValuesHelper.
func NewRuntimeChartValuesHelper(
	runtimeClient client.Client,
	virtualGarden bool,
	virtualGardenKubeconfigGardenerAPIServer *string,
	virtualGardenKubeconfigGardenerControllerManager *string,
	virtualGardenKubeconfigGardenerScheduler *string,
	virtualGardenKubeconfigGardenerAdmissionController *string,
	admissionControllerConfig *admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration,
	controllerManagerConfig *controllermanagerconfigv1alpha1.ControllerManagerConfiguration,
	schedulerConfig *schedulerconfigv1alpha1.SchedulerConfiguration,
) RuntimeChartValuesHelper {
	return &runtimeValuesHelper{
		RuntimeClient:                            runtimeClient,
		VirtualGarden:                            virtualGarden,
		VirtualGardenKubeconfigGardenerAPIServer: virtualGardenKubeconfigGardenerAPIServer,
		VirtualGardenKubeconfigGardenerControllerManager:   virtualGardenKubeconfigGardenerControllerManager,
		VirtualGardenKubeconfigGardenerScheduler:           virtualGardenKubeconfigGardenerScheduler,
		VirtualGardenKubeconfigGardenerAdmissionController: virtualGardenKubeconfigGardenerAdmissionController,
		AdmissionControllerConfig:                          admissionControllerConfig,
		ControllerManagerConfig:                            controllerManagerConfig,
		SchedulerConfig:                                    schedulerConfig,
	}
}

func (v runtimeValuesHelper) GetRuntimeChartValues() (map[string]interface{}, error) {
	var (
		values = make(map[string]interface{})
		err    error
	)

	values, err = utils.SetToValuesMap(values, v.VirtualGarden, "deployment", "virtualGarden", "enabled")
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"global": values,
	}, nil
}
