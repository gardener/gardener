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

	"github.com/Masterminds/semver"
	"github.com/go-logr/logr"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
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
	c, err := r.instantiateComponents(ctx, log, garden, secretsManager, targetVersion, kubernetes.NewApplier(r.RuntimeClientSet.Client(), r.RuntimeClientSet.Client().RESTMapper()))
	if err != nil {
		return reconcile.Result{}, err
	}

	var (
		g = flow.NewGraph("Garden deletion")

		_ = g.Add(flow.Task{
			Name: "Destroying Kube State Metrics",
			Fn:   component.OpDestroyAndWait(c.kubeStateMetrics).Destroy,
		})
		destroyVirtualGardenGardenerAccess = g.Add(flow.Task{
			Name: "Destroying Gardener virtual garden access resources",
			Fn:   c.virtualGardenGardenerAccess.Destroy,
		})
		destroyVirtualGardenGardenerResourceManager = g.Add(flow.Task{
			Name:         "Destroying gardener-resource-manager for virtual garden",
			Fn:           component.OpDestroyAndWait(c.virtualGardenGardenerResourceManager).Destroy,
			Dependencies: flow.NewTaskIDs(destroyVirtualGardenGardenerAccess),
		})
		destroyKubeControllerManager = g.Add(flow.Task{
			Name: "Destroying Kubernetes Controller Manager Server",
			Fn:   component.OpDestroyAndWait(c.kubeControllerManager).Destroy,
		})
		destroyKubeAPIServerSNI = g.Add(flow.Task{
			Name: "Destroying Kubernetes API server service SNI",
			Fn:   component.OpDestroyAndWait(c.kubeAPIServerSNI).Destroy,
		})
		destroyKubeAPIServerService = g.Add(flow.Task{
			Name:         "Destroying Kubernetes API Server service",
			Fn:           component.OpDestroyAndWait(c.kubeAPIServerService).Destroy,
			Dependencies: flow.NewTaskIDs(destroyKubeAPIServerSNI),
		})
		destroyKubeAPIServer = g.Add(flow.Task{
			Name: "Destroying Kubernetes API Server",
			Fn:   component.OpDestroyAndWait(c.kubeAPIServer).Destroy,
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
		syncPointVirtualGardenControlPlaneDestroyed = flow.NewTaskIDs(
			cleanupGenericTokenKubeconfig,
			destroyVirtualGardenGardenerAccess,
			destroyVirtualGardenGardenerResourceManager,
			destroyKubeControllerManager,
			destroyKubeAPIServerSNI,
			destroyKubeAPIServerService,
			destroyKubeAPIServer,
			destroyEtcd,
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
		syncPointCleanedUp = flow.NewTaskIDs(
			destroyEtcdDruid,
			destroyIstio,
			destroyHVPAController,
			destroyVerticalPodAutoscaler,
		)

		destroySystemResources = g.Add(flow.Task{
			Name:         "Destroying system resources",
			Fn:           component.OpDestroyAndWait(c.system).Destroy,
			Dependencies: flow.NewTaskIDs(syncPointCleanedUp),
		})
		ensureNoManagedResourcesExistAnymore = g.Add(flow.Task{
			Name:         "Ensuring no ManagedResources exist anymore",
			Fn:           r.checkIfManagedResourcesExist(),
			Dependencies: flow.NewTaskIDs(destroySystemResources),
		})
		destroyGardenerResourceManager = g.Add(flow.Task{
			Name:         "Destroying and waiting for gardener-resource-manager to be deleted",
			Fn:           component.OpWait(c.gardenerResourceManager).Destroy,
			Dependencies: flow.NewTaskIDs(ensureNoManagedResourcesExistAnymore),
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
			Name:         "Cleaning up secrets",
			Fn:           secretsManager.Cleanup,
			Dependencies: flow.NewTaskIDs(destroyGardenerResourceManager),
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log:              log,
		ProgressReporter: r.reportProgress(log, garden),
	}); err != nil {
		return reconcilerutils.ReconcileErr(flow.Errors(err))
	}

	// TODO(rfranzke): Remove this block in a future version (after v1.72 is released).
	{
		objects := []client.Object{
			&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "etcd-to-world", Namespace: r.GardenNamespace}},
			&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-allow-all", Namespace: r.GardenNamespace}},
			&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "istio-allow-all", Namespace: c.istio.GetValues().Istiod.Namespace}},
		}
		for _, istioIngress := range c.istio.GetValues().IngressGateway {
			objects = append(objects, &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "istio-allow-all", Namespace: istioIngress.Namespace}})
		}
		if err := kubernetesutils.DeleteObjects(ctx, r.RuntimeClientSet.Client(), objects...); err != nil {
			return reconcile.Result{}, err
		}
	}

	if controllerutil.ContainsFinalizer(garden, finalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.RuntimeClientSet.Client(), garden, finalizerName); err != nil {
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
