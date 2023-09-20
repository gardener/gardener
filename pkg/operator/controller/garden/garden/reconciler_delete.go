// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package garden

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

func (r *Reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	garden *operatorv1alpha1.Garden,
	secretsManager secretsmanager.Interface,
	targetVersion *semver.Version,
) (
	reconcile.Result,
	error,
) {
	log.Info("Instantiating component destroyers")
	c, err := r.instantiateComponents(ctx, log, garden, secretsManager, targetVersion, kubernetes.NewApplier(r.RuntimeClientSet.Client(), r.RuntimeClientSet.Client().RESTMapper()), nil, false)
	if err != nil {
		return reconcile.Result{}, err
	}

	var (
		g = flow.NewGraph("Garden deletion")

		_ = g.Add(flow.Task{
			Name: "Destroying Plutono",
			Fn:   component.OpDestroyAndWait(c.plutono).Destroy,
		})
		_ = g.Add(flow.Task{
			Name: "Destroying Gardener Metrics Exporter",
			Fn:   component.OpDestroyAndWait(c.gardenerMetricsExporter).Destroy,
		})
		_ = g.Add(flow.Task{
			Name: "Destroying Kube State Metrics",
			Fn:   component.OpDestroyAndWait(c.kubeStateMetrics).Destroy,
		})

		destroyGardenerScheduler = g.Add(flow.Task{
			Name: "Destroying Gardener Scheduler",
			Fn:   component.OpDestroyAndWait(c.gardenerScheduler).Destroy,
		})
		destroyGardenerControllerManager = g.Add(flow.Task{
			Name: "Destroying Gardener Controller Manager",
			Fn:   component.OpDestroyAndWait(c.gardenerControllerManager).Destroy,
		})
		destroyGardenerAdmissionController = g.Add(flow.Task{
			Name: "Destroying Gardener Admission Controller",
			Fn:   component.OpDestroyAndWait(c.gardenerAdmissionController).Destroy,
		})
		destroyGardenerAPIServer = g.Add(flow.Task{
			Name: "Destroying Gardener API Server",
			Fn:   component.OpDestroyAndWait(c.gardenerAPIServer).Destroy,
		})
		destroyVirtualSystemResources = g.Add(flow.Task{
			Name:         "Destroying virtual system resources",
			Fn:           component.OpDestroyAndWait(c.virtualSystem).Destroy,
			Dependencies: flow.NewTaskIDs(destroyGardenerAPIServer),
		})

		destroyVirtualGardenGardenerAccess = g.Add(flow.Task{
			Name: "Destroying Gardener virtual garden access resources",
			Fn:   component.OpDestroyAndWait(c.virtualGardenGardenerAccess).Destroy,
		})
		destroyKubeControllerManager = g.Add(flow.Task{
			Name: "Destroying Kubernetes Controller Manager Server",
			Fn:   component.OpDestroyAndWait(c.kubeControllerManager).Destroy,
		})
		syncPointVirtualGardenManagedResourcesDestroyed = flow.NewTaskIDs(
			destroyGardenerScheduler,
			destroyGardenerControllerManager,
			destroyGardenerAdmissionController,
			destroyGardenerAPIServer,
			destroyVirtualSystemResources,
			destroyVirtualGardenGardenerAccess,
			destroyKubeControllerManager,
		)

		destroyVirtualGardenGardenerResourceManager = g.Add(flow.Task{
			Name:         "Destroying gardener-resource-manager for virtual garden",
			Fn:           component.OpDestroyAndWait(c.virtualGardenGardenerResourceManager).Destroy,
			Dependencies: flow.NewTaskIDs(syncPointVirtualGardenManagedResourcesDestroyed),
		})
		destroyKubeAPIServerSNI = g.Add(flow.Task{
			Name:         "Destroying Kubernetes API server service SNI",
			Fn:           component.OpDestroyAndWait(c.kubeAPIServerSNI).Destroy,
			Dependencies: flow.NewTaskIDs(destroyVirtualGardenGardenerResourceManager),
		})
		destroyKubeAPIServerService = g.Add(flow.Task{
			Name:         "Destroying Kubernetes API Server service",
			Fn:           component.OpDestroyAndWait(c.kubeAPIServerService).Destroy,
			Dependencies: flow.NewTaskIDs(destroyKubeAPIServerSNI),
		})
		destroyKubeAPIServer = g.Add(flow.Task{
			Name:         "Destroying Kubernetes API Server",
			Fn:           component.OpDestroyAndWait(c.kubeAPIServer).Destroy,
			Dependencies: flow.NewTaskIDs(destroyVirtualGardenGardenerResourceManager),
		})
		destroyEtcd = g.Add(flow.Task{
			Name: "Destroying main and events ETCDs of virtual garden",
			Fn: flow.Parallel(
				component.OpDestroyAndWait(c.etcdMain).Destroy,
				component.OpDestroyAndWait(c.etcdEvents).Destroy,
			),
			Dependencies: flow.NewTaskIDs(destroyKubeAPIServer),
		})
		cleanupGenericTokenKubeconfig = g.Add(flow.Task{
			Name:         "Cleaning up generic token kubeconfig",
			Fn:           func(ctx context.Context) error { return r.cleanupGenericTokenKubeconfig(ctx, secretsManager) },
			Dependencies: flow.NewTaskIDs(destroyKubeAPIServer, destroyVirtualGardenGardenerResourceManager),
		})
		invalidateClient = g.Add(flow.Task{
			Name: "Invalidate client for virtual garden",
			Fn: func(ctx context.Context) error {
				return r.GardenClientMap.InvalidateClient(keys.ForGarden(garden))
			},
			Dependencies: flow.NewTaskIDs(destroyKubeAPIServer, destroyVirtualGardenGardenerResourceManager),
		})
		syncPointVirtualGardenControlPlaneDestroyed = flow.NewTaskIDs(
			cleanupGenericTokenKubeconfig,
			destroyVirtualGardenGardenerResourceManager,
			destroyKubeAPIServerSNI,
			destroyKubeAPIServerService,
			destroyKubeAPIServer,
			destroyEtcd,
			invalidateClient,
		)

		destroyEtcdDruid = g.Add(flow.Task{
			Name:         "Destroying ETCD Druid",
			Fn:           component.OpDestroyAndWait(c.etcdDruid).Destroy,
			Dependencies: flow.NewTaskIDs(syncPointVirtualGardenControlPlaneDestroyed),
		})
		destroyIstio = g.Add(flow.Task{
			Name: "Destroying Istio",
			Fn:   component.OpDestroyAndWait(c.istio).Destroy,
		})
		destroyHVPAController = g.Add(flow.Task{
			Name:         "Destroying HVPA controller",
			Fn:           component.OpDestroyAndWait(c.hvpaController).Destroy,
			Dependencies: flow.NewTaskIDs(syncPointVirtualGardenControlPlaneDestroyed),
		})
		destroyVerticalPodAutoscaler = g.Add(flow.Task{
			Name:         "Destroying Kubernetes vertical pod autoscaler",
			Fn:           component.OpDestroyAndWait(c.verticalPodAutoscaler).Destroy,
			Dependencies: flow.NewTaskIDs(syncPointVirtualGardenControlPlaneDestroyed),
		})
		destroyNginxIngressController = g.Add(flow.Task{
			Name:         "Destroying nginx-ingress controller",
			Fn:           component.OpDestroyAndWait(c.nginxIngressController).Destroy,
			Dependencies: flow.NewTaskIDs(syncPointVirtualGardenControlPlaneDestroyed),
		})
		destroyFluentOperatorCustomResources = g.Add(flow.Task{
			Name:         "Destroying fluent-operator custom resources",
			Fn:           component.OpDestroyAndWait(c.fluentOperatorCustomResources).Destroy,
			Dependencies: flow.NewTaskIDs(syncPointVirtualGardenControlPlaneDestroyed),
		})
		destroyFluentBit = g.Add(flow.Task{
			Name:         "Destroying fluent-bit",
			Fn:           component.OpDestroyAndWait(c.fluentBit).Destroy,
			Dependencies: flow.NewTaskIDs(syncPointVirtualGardenControlPlaneDestroyed),
		})
		destroyFluentOperator = g.Add(flow.Task{
			Name:         "Destroying fluent-operator",
			Fn:           component.OpDestroyAndWait(c.fluentOperator).Destroy,
			Dependencies: flow.NewTaskIDs(destroyFluentOperatorCustomResources, destroyFluentBit),
		})
		destroyVali = g.Add(flow.Task{
			Name:         "Destroying Vali",
			Fn:           component.OpDestroyAndWait(c.vali).Destroy,
			Dependencies: flow.NewTaskIDs(destroyFluentOperatorCustomResources),
		})
		syncPointCleanedUp = flow.NewTaskIDs(
			destroyEtcdDruid,
			destroyIstio,
			destroyHVPAController,
			destroyVerticalPodAutoscaler,
			destroyNginxIngressController,
			destroyFluentOperatorCustomResources,
			destroyFluentBit,
			destroyFluentOperator,
			destroyVali,
		)

		destroyRuntimeSystemResources = g.Add(flow.Task{
			Name:         "Destroying runtime system resources",
			Fn:           component.OpDestroyAndWait(c.runtimeSystem).Destroy,
			Dependencies: flow.NewTaskIDs(syncPointCleanedUp),
		})
		ensureNoManagedResourcesExistAnymore = g.Add(flow.Task{
			Name:         "Ensuring no ManagedResources exist anymore",
			Fn:           r.checkIfManagedResourcesExist(),
			Dependencies: flow.NewTaskIDs(destroyRuntimeSystemResources),
		})
		destroyGardenerResourceManager = g.Add(flow.Task{
			Name:         "Destroying and waiting for gardener-resource-manager to be deleted",
			Fn:           component.OpWait(c.gardenerResourceManager).Destroy,
			Dependencies: flow.NewTaskIDs(ensureNoManagedResourcesExistAnymore),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying custom resource definition for fluent-operator",
			Fn:           c.fluentCRD.Destroy,
			Dependencies: flow.NewTaskIDs(destroyGardenerResourceManager),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying custom resource definition for Istio",
			Fn:           c.istioCRD.Destroy,
			Dependencies: flow.NewTaskIDs(destroyGardenerResourceManager),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying custom resource definition for VPA",
			Fn:           flow.TaskFn(c.vpaCRD.Destroy).DoIf(vpaEnabled(garden.Spec.RuntimeCluster.Settings)),
			Dependencies: flow.NewTaskIDs(destroyGardenerResourceManager),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying custom resource definition for HVPA",
			Fn:           flow.TaskFn(c.hvpaCRD.Destroy).DoIf(hvpaEnabled()),
			Dependencies: flow.NewTaskIDs(destroyGardenerResourceManager),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying ETCD-related custom resource definitions",
			Fn:           c.etcdCRD.Destroy,
			Dependencies: flow.NewTaskIDs(destroyGardenerResourceManager),
		})
		_ = g.Add(flow.Task{
			Name:         "Cleaning up secrets",
			Fn:           secretsManager.Cleanup,
			Dependencies: flow.NewTaskIDs(destroyGardenerResourceManager),
		})
	)

	gardenCopy := garden.DeepCopy()
	if err := g.Compile().Run(ctx, flow.Opts{
		Log:              log,
		ProgressReporter: r.reportProgress(log, gardenCopy),
	}); err != nil {
		return reconcilerutils.ReconcileErr(flow.Errors(err))
	}
	*garden = *gardenCopy

	if controllerutil.ContainsFinalizer(garden, operatorv1alpha1.FinalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.RuntimeClientSet.Client(), garden, operatorv1alpha1.FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) checkIfManagedResourcesExist() func(context.Context) error {
	return func(ctx context.Context) error {
		managedResourcesStillExist, err := managedresources.CheckIfManagedResourcesExist(ctx, r.RuntimeClientSet.Client(), pointer.String(v1beta1constants.SeedResourceManagerClass))
		if err != nil {
			return err
		}

		if !managedResourcesStillExist {
			return nil
		}

		return &reconcilerutils.RequeueAfterError{
			RequeueAfter: 5 * time.Second,
			Cause:        errors.New("at least one ManagedResource still exists"),
		}
	}
}
