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
	"strings"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionsconfig "github.com/gardener/gardener/extensions/pkg/apis/config"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/worker"
	"github.com/gardener/gardener/extensions/pkg/controller/worker/genericactuator"
	"github.com/gardener/gardener/extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/helper"
	localimagevector "github.com/gardener/gardener/pkg/provider-local/imagevector"
	"github.com/gardener/gardener/pkg/provider-local/local"
	"github.com/gardener/gardener/pkg/utils/chart"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

type delegateFactory struct {
	client     client.Client
	decoder    runtime.Decoder
	restConfig *rest.Config
	scheme     *runtime.Scheme
}

type actuator struct {
	worker.Actuator
	workerDelegate *delegateFactory
}

// NewActuator creates a new Actuator that updates the status of the handled WorkerPoolConfigs.
func NewActuator(mgr manager.Manager, gardenletManagesMCM bool) (worker.Actuator, error) {
	var (
		mcmName              string
		mcmChartSeed         *chart.Chart
		mcmChartShoot        *chart.Chart
		imageVector          imagevector.ImageVector
		chartRendererFactory extensionscontroller.ChartRendererFactory
		workerDelegate       = &delegateFactory{
			client:     mgr.GetClient(),
			decoder:    serializer.NewCodecFactory(mgr.GetScheme(), serializer.EnableStrict).UniversalDecoder(),
			restConfig: mgr.GetConfig(),
			scheme:     mgr.GetScheme(),
		}
	)

	if !gardenletManagesMCM {
		mcmName = local.MachineControllerManagerName
		mcmChartSeed = mcmChart
		mcmChartShoot = mcmShootChart
		imageVector = localimagevector.ImageVector()
		chartRendererFactory = extensionscontroller.ChartRendererFactoryFunc(util.NewChartRendererForShoot)
	}

	genericactuator, err := genericactuator.NewActuator(
		mgr,
		workerDelegate,
		mcmName,
		mcmChartSeed,
		mcmChartShoot,
		imageVector,
		chartRendererFactory,
		nil,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return &actuator{
		Actuator:       genericactuator,
		workerDelegate: workerDelegate,
	}, nil
}

func (a *actuator) Restore(ctx context.Context, log logr.Logger, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) error {
	if err := genericactuator.RestoreWithoutReconcile(ctx, log, a.workerDelegate.client, a.workerDelegate, worker, cluster); err != nil {
		return fmt.Errorf("failed restoring the worker state: %w", err)
	}

	// At this point, the generic actuator has restored all Machine objects into the shoot namespace of the new
	// destination seed. However, in the local scenario, the shoot worker nodes are not really external machines but
	// "internal" pods running next to the control plane in the seed.
	// Since the pods cannot be migrated from the source seed to the destination seed, the shoot worker node pods cannot
	// be restored. Instead, they have to be recreated.
	// In order to trigger this recreation, we are deleting all (restored) machines which are no longer backed by any
	// pods now. We also delete the corresponding Node objects in the shoot. The MCM's MachineSet controller will
	// automatically recreate new Machines now, which in fact will result in new pods and nodes.
	// In summary, we are still not simulating the very same CPM scenario as for real clouds (here, the nodes/VMs are
	// external and remain during the migration), but this is as good as we can get for the local scenario.
	if err := a.deleteNoLongerNeededMachines(ctx, log, worker.Namespace); err != nil {
		return fmt.Errorf("failed deleting no longer existing machines after restoration: %w", err)
	}

	return a.Actuator.Reconcile(ctx, log, worker, cluster)
}

func (a *actuator) deleteNoLongerNeededMachines(ctx context.Context, log logr.Logger, namespace string) error {
	_, shootClient, err := util.NewClientForShoot(ctx, a.workerDelegate.client, namespace, client.Options{}, extensionsconfig.RESTOptions{})
	if err != nil {
		return fmt.Errorf("failed creating client for shoot cluster: %w", err)
	}

	machineList := &machinev1alpha1.MachineList{}
	if err := a.workerDelegate.client.List(ctx, machineList, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("failed listing machines: %w", err)
	}

	podList := &corev1.PodList{}
	if err := a.workerDelegate.client.List(ctx, podList, client.InNamespace(namespace), client.MatchingLabels{"app": "machine"}); err != nil {
		return fmt.Errorf("failed listing pods: %w", err)
	}

	machineNameToPodName := make(map[string]string)
	for _, pod := range podList.Items {
		machineNameToPodName[strings.TrimPrefix(pod.Name, "machine-")] = pod.Name
	}

	for _, machine := range machineList.Items {
		if _, ok := machineNameToPodName[machine.Name]; ok {
			continue
		}

		log.Info("Deleting machine since it is not backed by any pod", "machine", client.ObjectKeyFromObject(machine.DeepCopy()))

		nodeName := "machine-" + machine.Name
		if err := shootClient.Delete(ctx, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed deleting node %q for machine %q: %w", nodeName, machine.Name, err)
		}

		if err := a.workerDelegate.client.Delete(ctx, machine.DeepCopy()); err != nil {
			return fmt.Errorf("failed deleting machine %q: %w", machine.Name, err)
		}
	}

	return nil
}

func (d *delegateFactory) WorkerDelegate(_ context.Context, worker *extensionsv1alpha1.Worker, cluster *extensionscontroller.Cluster) (genericactuator.WorkerDelegate, error) {
	clientset, err := kubernetes.NewForConfig(d.restConfig)
	if err != nil {
		return nil, err
	}

	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, err
	}

	seedChartApplier, err := kubernetesclient.NewChartApplierForConfig(d.restConfig)
	if err != nil {
		return nil, err
	}

	return NewWorkerDelegate(
		d.client,
		d.decoder,
		d.scheme,
		seedChartApplier,
		serverVersion.GitVersion,
		worker,
		cluster,
	)
}

type workerDelegate struct {
	client  client.Client
	decoder runtime.Decoder
	scheme  *runtime.Scheme

	seedChartApplier    kubernetesclient.ChartApplier
	serverVersion       string
	cloudProfileConfig  *api.CloudProfileConfig
	cluster             *extensionscontroller.Cluster
	worker              *extensionsv1alpha1.Worker
	machineClassSecrets []*corev1.Secret
	machineClasses      []*machinev1alpha1.MachineClass
	machineImages       []api.MachineImage
	machineDeployments  worker.MachineDeployments
}

// NewWorkerDelegate creates a new context for a worker reconciliation.
func NewWorkerDelegate(
	client client.Client,
	decoder runtime.Decoder,
	scheme *runtime.Scheme,
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
		scheme:             scheme,
		client:             client,
		decoder:            decoder,
		seedChartApplier:   seedChartApplier,
		serverVersion:      serverVersion,
		cloudProfileConfig: config,
		cluster:            cluster,
		worker:             worker,
	}, nil
}
