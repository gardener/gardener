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
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/common"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	"github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/helper"
	"github.com/gardener/gardener/pkg/provider-local/imagevector"
	"github.com/gardener/gardener/pkg/provider-local/local"
)

type delegateFactory struct {
	common.RESTConfigContext
}

type actuator struct {
	worker.Actuator
}

// NewActuator creates a new Actuator that updates the status of the handled WorkerPoolConfigs.
func NewActuator() worker.Actuator {
	delegateFactory := &delegateFactory{}

	return &actuator{
		genericactuator.NewActuator(
			delegateFactory,
			local.MachineControllerManagerName,
			mcmChart,
			mcmShootChart,
			imagevector.ImageVector(),
			extensionscontroller.ChartRendererFactoryFunc(util.NewChartRendererForShoot),
		),
	}
}

func (a *actuator) InjectFunc(f inject.Func) error {
	return f(a.Actuator)
}

func (a *actuator) Restore(ctx context.Context, log logr.Logger, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) error {
	if err := genericactuator.RestoreWithoutReconcile(ctx, log, a.workerDelegate.Client(), a.workerDelegate, worker, cluster); err != nil {
		return fmt.Errorf("failed restoring the worker state: %w", err)
	}

	return a.Actuator.Reconcile(ctx, log, worker, cluster)
}

func (d *delegateFactory) WorkerDelegate(_ context.Context, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) (genericactuator.WorkerDelegate, error) {
	clientset, err := kubernetes.NewForConfig(d.RESTConfig())
	if err != nil {
		return nil, err
	}

	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	seedChartApplier, err := kubernetesclient.NewChartApplierForConfig(d.RESTConfig())
	if err != nil {
		return nil, err
	}

	return NewWorkerDelegate(
		d.ClientContext,
		seedChartApplier,
		serverVersion.GitVersion,
		worker,
		cluster,
	)
}

type workerDelegate struct {
	common.ClientContext
	seedChartApplier   kubernetesclient.ChartApplier
	serverVersion      string
	cloudProfileConfig *api.CloudProfileConfig
	cluster            *extensionscontroller.Cluster
	worker             *extensionsv1alpha1.Worker
	machineClasses     []map[string]interface{}
	machineDeployments worker.MachineDeployments
	machineImages      []api.MachineImage
}

// NewWorkerDelegate creates a new context for a worker reconciliation.
func NewWorkerDelegate(
	clientContext common.ClientContext,
	seedChartApplier kubernetesclient.ChartApplier,
	serverVersion string,
	worker *extensionsv1alpha1.Worker,
	cluster *extensionscontroller.Cluster,
) (
	genericactuator.WorkerDelegate,
	error,
) {
	config, err := helper.CloudProfileConfigFromCluster(cluster)
	if err != nil {
		return nil, err
	}

	return &workerDelegate{
		ClientContext:      clientContext,
		seedChartApplier:   seedChartApplier,
		serverVersion:      serverVersion,
		cloudProfileConfig: config,
		cluster:            cluster,
		worker:             worker,
	}, nil
}
