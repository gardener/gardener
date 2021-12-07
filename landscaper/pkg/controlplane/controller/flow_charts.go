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

package controller

import (
	"context"
	"fmt"

	importsv1alpha1 "github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/v1alpha1"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/controller/chart"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/controller/values"
	"k8s.io/utils/pointer"
)

const (
	// suffixApplicationChart is the suffix in the application chart path
	suffixApplicationChart = "application"

	// suffixRuntimeChart is the suffix in the runtime chart path
	suffixRuntimeChart = "runtime"
)

// DeployApplicationChart deploys the application chart into the Garden cluster
func (o *operation) DeployApplicationChart(ctx context.Context) error {
	values, err := o.getAplicationChartValues()
	if err != nil {
		return err
	}

	applier := chart.NewChartApplier(o.getGardenClient().ChartApplier(), values, o.chartPath, suffixApplicationChart)
	if err := applier.Deploy(ctx); err != nil {
		return fmt.Errorf("failed deploying control plane application chart to the Garden cluster: %w", err)
	}

	return nil
}

// DestroyApplicationChart destroy the application chart from the Garden cluster
func (o *operation) DestroyApplicationChart(ctx context.Context) error {
	values, err := o.getAplicationChartValues()
	if err != nil {
		return err
	}

	applier := chart.NewChartApplier(o.getGardenClient().ChartApplier(), values, o.chartPath, suffixApplicationChart)
	if err := applier.Destroy(ctx); err != nil {
		return fmt.Errorf("failed to destroy the control plane application chart from the Garden cluster: %w", err)
	}

	return nil
}

func (o *operation) getAplicationChartValues() (map[string]interface{}, error) {
	var (
		cgClusterIP                   *string
		gardenerAdmissionControllerCA *string
		seedRestrictionEnabled        *bool
	)

	if o.imports.VirtualGarden != nil && o.imports.VirtualGarden.Enabled {
		cgClusterIP = o.imports.VirtualGarden.ClusterIP
	}

	if o.imports.GardenerAdmissionController.Enabled {
		gardenerAdmissionControllerCA = o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Crt
	}

	if o.imports.GardenerAdmissionController.SeedRestriction != nil {
		seedRestrictionEnabled = pointer.Bool(o.imports.GardenerAdmissionController.SeedRestriction.Enabled)
	}

	valuesHelper := values.NewApplicationChartValuesHelper(
		o.imports.VirtualGarden != nil && o.imports.VirtualGarden.Enabled,
		cgClusterIP,
		*o.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt,
		gardenerAdmissionControllerCA,
		o.imports.InternalDomain,
		o.imports.DefaultDomains,
		*o.imports.OpenVPNDiffieHellmanKey,
		o.imports.Alerting,
		o.admissionControllerConfig,
		seedRestrictionEnabled,
	)

	values, err := valuesHelper.GetApplicationChartValues()
	if err != nil {
		return nil, fmt.Errorf("failed to generate the values for the control plane application chart: %w", err)
	}
	return values, nil
}

// DeployRuntimeChart deploys the runtime chart into the runtime cluster
func (o *operation) DeployRuntimeChart(ctx context.Context) error {
	values, err := o.getRuntimeChartValues()
	if err != nil {
		return err
	}

	applier := chart.NewChartApplier(o.runtimeClient.ChartApplier(), values, o.chartPath, suffixRuntimeChart)
	if err := applier.Deploy(ctx); err != nil {
		return fmt.Errorf("failed deploying control plane runtime chart to the runtime cluster: %w", err)
	}

	return nil
}

// DestroyRuntimeChart deletes the runtime chart from the runtime cluster
func (o *operation) DestroyRuntimeChart(ctx context.Context) error {
	values, err := o.getRuntimeChartValues()
	if err != nil {
		return err
	}

	applier := chart.NewChartApplier(o.runtimeClient.ChartApplier(), values, o.chartPath, suffixRuntimeChart)
	if err := applier.Destroy(ctx); err != nil {
		return fmt.Errorf("failed to destroy the control plane runtime chart from the runtime cluster: %w", err)
	}

	return nil
}

func (o *operation) getRuntimeChartValues() (map[string]interface{}, error) {
	var cgClusterIP *string

	if o.imports.VirtualGarden != nil && o.imports.VirtualGarden.Enabled {
		cgClusterIP = o.imports.VirtualGarden.ClusterIP
	}

	// work on external version of the import configuration for marshalling into values
	// internal structs do not have json marshalling tags
	gardenerAPIServerV1alpha1 := &importsv1alpha1.GardenerAPIServer{}
	if err := importsv1alpha1.Convert_imports_GardenerAPIServer_To_v1alpha1_GardenerAPIServer(&o.imports.GardenerAPIServer, gardenerAPIServerV1alpha1, nil); err != nil {
		return nil, fmt.Errorf("failed to convert Gardern API Server import configuration to external version for rendering the chart values: %w", err)
	}

	gardenerControllerManagerV1alpha1 := &importsv1alpha1.GardenerControllerManager{}
	if err := importsv1alpha1.Convert_imports_GardenerControllerManager_To_v1alpha1_GardenerControllerManager(o.imports.GardenerControllerManager, gardenerControllerManagerV1alpha1, nil); err != nil {
		return nil, fmt.Errorf("failed to convert Gardener Controller Manager import configuration to external version for rendering the chart values: %w", err)
	}

	gardenerSchedulerV1alpha1 := &importsv1alpha1.GardenerScheduler{}
	if err := importsv1alpha1.Convert_imports_GardenerScheduler_To_v1alpha1_GardenerScheduler(o.imports.GardenerScheduler, gardenerSchedulerV1alpha1, nil); err != nil {
		return nil, fmt.Errorf("failed to convert Gardener Scheduler import configuration to external version for rendering the chart values: %w", err)
	}

	gardenerAdmissionControllerV1alpha1 := &importsv1alpha1.GardenerAdmissionController{}
	if err := importsv1alpha1.Convert_imports_GardenerAdmissionController_To_v1alpha1_GardenerAdmissionController(o.imports.GardenerAdmissionController, gardenerAdmissionControllerV1alpha1, nil); err != nil {
		return nil, fmt.Errorf("failed to convert Gardener Admission Controller import configuration to external version for rendering the chart values: %w", err)
	}

	rbacV1alpha1 := &importsv1alpha1.Rbac{}
	if o.imports.Rbac != nil {
		if err := importsv1alpha1.Convert_imports_Rbac_To_v1alpha1_Rbac(o.imports.Rbac, rbacV1alpha1, nil); err != nil {
			return nil, fmt.Errorf("failed to convert RBAC import configuration to external version for rendering the chart values: %w", err)
		}
	}

	valuesHelper := values.NewRuntimeChartValuesHelper(
		*o.imports.Identity,
		o.imports.VirtualGarden != nil && o.imports.VirtualGarden.Enabled,
		rbacV1alpha1,
		cgClusterIP,
		o.VirtualGardenKubeconfigGardenerAPIServer,
		o.VirtualGardenKubeconfigGardenerControllerManager,
		o.VirtualGardenKubeconfigGardenerScheduler,
		o.VirtualGardenKubeconfigGardenerAdmissionController,
		o.admissionControllerConfig,
		o.controllerManagerConfig,
		o.schedulerConfig,
		*gardenerAPIServerV1alpha1,
		*gardenerControllerManagerV1alpha1,
		*gardenerAdmissionControllerV1alpha1,
		*gardenerSchedulerV1alpha1,
		values.Image{
			Repository: o.ImageReferences.ApiServer.Repository,
			Tag:        o.ImageReferences.ApiServer.Tag,
		},
		values.Image{
			Repository: o.ImageReferences.ControllerManager.Repository,
			Tag:        o.ImageReferences.ControllerManager.Tag,
		},
		values.Image{
			Repository: o.ImageReferences.Scheduler.Repository,
			Tag:        o.ImageReferences.Scheduler.Tag,
		},
		values.Image{
			Repository: o.ImageReferences.AdmissionController.Repository,
			Tag:        o.ImageReferences.AdmissionController.Tag,
		},
	)

	values, err := valuesHelper.GetRuntimeChartValues()
	if err != nil {
		return nil, fmt.Errorf("failed to generate the values for the control plane runtime chart: %w", err)
	}
	return values, nil
}
