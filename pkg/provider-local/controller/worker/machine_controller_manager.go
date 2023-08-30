// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package worker

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/provider-local/charts"
	localimagevector "github.com/gardener/gardener/pkg/provider-local/imagevector"
	"github.com/gardener/gardener/pkg/provider-local/local"
	"github.com/gardener/gardener/pkg/utils/chart"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var (
	mcmChart = &chart.Chart{
		Name:       local.MachineControllerManagerName,
		EmbeddedFS: &charts.ChartMachineControllerManagerSeed,
		Path:       charts.ChartPathMachineControllerManagerSeed,
		Images:     []string{localimagevector.ImageNameMachineControllerManager, localimagevector.ImageNameMachineControllerManagerProviderLocal},
		Objects: []*chart.Object{
			{Type: &appsv1.Deployment{}, Name: local.MachineControllerManagerName},
			{Type: &corev1.Service{}, Name: local.MachineControllerManagerName},
			{Type: &corev1.ServiceAccount{}, Name: local.MachineControllerManagerName},
			{Type: &corev1.Secret{}, Name: local.MachineControllerManagerName},
			{Type: extensionscontroller.GetVerticalPodAutoscalerObject(), Name: local.MachineControllerManagerVpaName},
			{Type: &corev1.ConfigMap{}, Name: local.MachineControllerManagerMonitoringConfigName},
		},
	}

	mcmShootChart = &chart.Chart{
		Name:       local.MachineControllerManagerName,
		EmbeddedFS: &charts.ChartMachineControllerManagerShoot,
		Path:       charts.ChartPathMachineControllerManagerShoot,
	}
)

func (w *workerDelegate) GetMachineControllerManagerChartValues(ctx context.Context) (map[string]interface{}, error) {
	namespace := &corev1.Namespace{}
	if err := w.client.Get(ctx, kubernetesutils.Key(w.worker.Namespace), namespace); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"providerName": local.Name,
		"namespace": map[string]interface{}{
			"uid": namespace.UID,
		},
		"podLabels": map[string]interface{}{
			v1beta1constants.LabelPodMaintenanceRestart: "true",
		},
	}, nil
}

func (w *workerDelegate) GetMachineControllerManagerShootChartValues(_ context.Context) (map[string]interface{}, error) {
	return map[string]interface{}{
		"providerName": local.Name,
	}, nil
}
