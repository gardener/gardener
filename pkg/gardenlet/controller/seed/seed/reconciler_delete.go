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

package seed

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/clusteridentity"
	"github.com/gardener/gardener/pkg/controllerutils"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

func (r *Reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	seedObj *seedpkg.Seed,
	seedIsGarden bool,
) error {
	seed := seedObj.GetInfo()

	if !sets.New(seed.Finalizers...).Has(gardencorev1beta1.GardenerName) {
		return nil
	}

	// Before deletion, it has to be ensured that no Shoots nor BackupBuckets depend on the Seed anymore.
	// When this happens the controller will remove the finalizers from the Seed so that it can be garbage collected.
	parentLogMessage := "Can't delete Seed, because the following objects are still referencing it:"

	associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(ctx, r.GardenClient, seed)
	if err != nil {
		return err
	}

	if len(associatedShoots) > 0 {
		log.Info("Cannot delete Seed because the following Shoots are still referencing it", "shoots", associatedShoots)
		r.Recorder.Event(seed, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, fmt.Sprintf("%s Shoots=%v", parentLogMessage, associatedShoots))

		return errors.New("seed still has references")
	}

	if seed.Spec.Backup != nil {
		backupBucket := &gardencorev1beta1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: string(seed.UID)}}

		if err := r.GardenClient.Delete(ctx, backupBucket); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	associatedBackupBuckets, err := controllerutils.DetermineBackupBucketAssociations(ctx, r.GardenClient, seed.Name)
	if err != nil {
		return err
	}

	if len(associatedBackupBuckets) > 0 {
		log.Info("Cannot delete Seed because the following BackupBuckets are still referencing it", "backupBuckets", associatedBackupBuckets)
		r.Recorder.Event(seed, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, fmt.Sprintf("%s BackupBuckets=%v", parentLogMessage, associatedBackupBuckets))

		return errors.New("seed still has references")
	}

	log.Info("No Shoots or BackupBuckets are referencing the Seed, deletion accepted")

	if err := r.runDeleteSeedFlow(ctx, log, seedObj, seedIsGarden); err != nil {
		return err
	}

	// Remove finalizer from Seed
	if controllerutil.ContainsFinalizer(seed, gardencorev1beta1.GardenerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.GardenClient, seed, gardencorev1beta1.GardenerName); err != nil {
			return fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return nil
}

func (r *Reconciler) runDeleteSeedFlow(
	ctx context.Context,
	log logr.Logger,
	seed *seedpkg.Seed,
	seedIsGarden bool,
) error {
	log.Info("Instantiating component deployers")
	c, err := r.instantiateComponents(ctx, log, seed, nil, seedIsGarden, nil, nil, nil)
	if err != nil {
		return err
	}

	seedIsOriginOfClusterIdentity, err := clusteridentity.IsClusterIdentityEmptyOrFromOrigin(ctx, r.SeedClientSet.Client(), v1beta1constants.ClusterIdentityOriginSeed)
	if err != nil {
		return err
	}

	var (
		g = flow.NewGraph("Seed deletion")

		// Delete all ingress objects in garden namespace which are not created as part of ManagedResources. This can be
		// removed once all seed system components are deployed as part of ManagedResources.
		// See https://github.com/gardener/gardener/issues/6062 for details.
		_ = g.Add(flow.Task{
			Name: "Destroying all networkingv1.Ingress resources in the garden namespace",
			Fn: func(ctx context.Context) error {
				return r.SeedClientSet.Client().DeleteAllOf(ctx, &networkingv1.Ingress{}, client.InNamespace(r.GardenNamespace))
			},
		})
		// Use the managed resource for cluster-identity only if there is no cluster-identity config map in kube-system namespace from a different origin than seed.
		// This prevents gardenlet from deleting the config map accidentally on seed deletion when it was created by a different party (gardener-apiserver or shoot).
		destroyClusterIdentity = g.Add(flow.Task{
			Name:   "Destroying cluster-identity",
			Fn:     component.OpDestroyAndWait(c.clusterIdentity).Destroy,
			SkipIf: !seedIsOriginOfClusterIdentity,
		})

		destroyDNSRecord = g.Add(flow.Task{
			Name: "Destroying managed ingress DNS record (if existing)",
			Fn:   component.OpDestroyAndWait(c.ingressDNSRecord).Destroy,
		})
		noControllerInstallations = g.Add(flow.Task{
			Name:         "Ensuring no ControllerInstallations are left",
			Fn:           ensureNoControllerInstallations(r.GardenClient, seed.GetInfo().Name),
			Dependencies: flow.NewTaskIDs(destroyDNSRecord),
		})
		destroyCachePrometheus = g.Add(flow.Task{
			Name: "Destroying cache Prometheus",
			Fn:   c.cachePrometheus.Destroy,
		})
		destroySeedPrometheus = g.Add(flow.Task{
			Name: "Destroying seed Prometheus",
			Fn:   c.seedPrometheus.Destroy,
		})
		destroyAggregatePrometheus = g.Add(flow.Task{
			Name: "Destroying aggregate Prometheus",
			Fn:   c.aggregatePrometheus.Destroy,
		})
		destroyAlertManager = g.Add(flow.Task{
			Name: "Destroying AlertManager",
			Fn:   c.alertManager.Destroy,
		})
		destroyClusterAutoscaler = g.Add(flow.Task{
			Name: "Destroying cluster-autoscaler resources",
			Fn:   component.OpDestroyAndWait(c.clusterAutoscaler).Destroy,
		})
		destroyMachineControllerManager = g.Add(flow.Task{
			Name: "Destroying machine-controller-manager resources",
			Fn:   component.OpDestroyAndWait(c.machineControllerManager).Destroy,
		})
		destroyNginxIngress = g.Add(flow.Task{
			Name:   "Destroying nginx-ingress",
			Fn:     component.OpDestroyAndWait(c.nginxIngressController).Destroy,
			SkipIf: seedIsGarden,
		})
		destroyDWDWeeder = g.Add(flow.Task{
			Name: "Destroy dependency-watchdog-weeder",
			Fn:   component.OpDestroyAndWait(c.dwdWeeder).Destroy,
		})
		destroyDWDProber = g.Add(flow.Task{
			Name: "Destroy dependency-watchdog-prober",
			Fn:   component.OpDestroyAndWait(c.dwdProber).Destroy,
		})
		destroyKubeAPIServerIngress = g.Add(flow.Task{
			Name: "Destroy kube-apiserver ingress",
			Fn:   component.OpDestroyAndWait(c.kubeAPIServerIngress).Destroy,
		})
		destroyKubeAPIServerService = g.Add(flow.Task{
			Name: "Destroy kube-apiserver service",
			Fn:   component.OpDestroyAndWait(c.kubeAPIServerService).Destroy,
		})
		destroyVPNAuthzServer = g.Add(flow.Task{
			Name: "Destroy VPN authorization server",
			Fn:   component.OpDestroyAndWait(c.vpnAuthzServer).Destroy,
		})
		destroyIstio = g.Add(flow.Task{
			Name: "Destroy Istio",
			Fn:   component.OpDestroyAndWait(c.istio).Destroy,
		})
		destroyIstioCRDs = g.Add(flow.Task{
			Name:         "Destroy Istio CRDs",
			Fn:           component.OpDestroyAndWait(c.istioCRD).Destroy,
			SkipIf:       seedIsGarden,
			Dependencies: flow.NewTaskIDs(destroyIstio),
		})
		destroyMachineControllerManagerCRDs = g.Add(flow.Task{
			Name: "Destroy machine-controller-manager CRDs",
			Fn:   component.OpDestroyAndWait(c.machineCRD).Destroy,
		})
		destroyFluentOperatorResources = g.Add(flow.Task{
			Name: "Destroy Fluent Operator Custom Resources",
			Fn:   component.OpDestroyAndWait(c.fluentOperatorCustomResources).Destroy,
		})

		// When the seed is the garden cluster then these components are reconciled by the gardener-operator.
		destroyPlutono = g.Add(flow.Task{
			Name:   "Destroying plutono",
			Fn:     component.OpDestroyAndWait(c.plutono).Destroy,
			SkipIf: seedIsGarden,
		})
		destroyEtcdDruid = g.Add(flow.Task{
			Name: "Destroying etcd druid",
			Fn:   component.OpDestroyAndWait(c.etcdDruid).Destroy,
			// only destroy Etcd CRD once all extension controllers are gone, otherwise they might not be able to start
			// up again (e.g. after being evicted by VPA)
			// see https://github.com/gardener/gardener/issues/6487#issuecomment-1220597217
			Dependencies: flow.NewTaskIDs(noControllerInstallations),
			SkipIf:       seedIsGarden,
		})
		destroyVPA = g.Add(flow.Task{
			Name:   "Destroy Kubernetes vertical pod autoscaler",
			Fn:     component.OpDestroyAndWait(c.verticalPodAutoscaler).Destroy,
			SkipIf: seedIsGarden,
		})
		destroyHVPA = g.Add(flow.Task{
			Name:   "Destroy HVPA controller",
			Fn:     component.OpDestroyAndWait(c.hvpaController).Destroy,
			SkipIf: seedIsGarden,
		})
		destroyGardenerCustomMetrics = g.Add(flow.Task{
			Name: "Destroy gardener-custom-metrics",
			Fn:   component.OpDestroyAndWait(c.gardenerCustomMetrics).Destroy,
		})
		destroyKubeStateMetrics = g.Add(flow.Task{
			Name:   "Destroy kube-state-metrics",
			Fn:     component.OpDestroyAndWait(c.kubeStateMetrics).Destroy,
			SkipIf: seedIsGarden,
		})
		destroyPrometheusOperator = g.Add(flow.Task{
			Name:   "Destroy Prometheus Operator",
			Fn:     component.OpDestroyAndWait(c.prometheusOperator).Destroy,
			SkipIf: seedIsGarden,
		})
		destroyFluentBit = g.Add(flow.Task{
			Name:   "Destroy Fluent Bit",
			Fn:     component.OpDestroyAndWait(c.fluentBit).Destroy,
			SkipIf: seedIsGarden,
		})
		destroyFluentOperator = g.Add(flow.Task{
			Name:         "Destroy Fluent Operator",
			Fn:           component.OpDestroyAndWait(c.fluentOperator).Destroy,
			Dependencies: flow.NewTaskIDs(destroyFluentOperatorResources, destroyFluentBit),
			SkipIf:       seedIsGarden,
		})
		destroyVali = g.Add(flow.Task{
			Name:         "Destroy Vali",
			Fn:           component.OpDestroyAndWait(c.vali).Destroy,
			Dependencies: flow.NewTaskIDs(destroyFluentOperatorResources),
			SkipIf:       seedIsGarden,
		})
		destroyEtcdCRD = g.Add(flow.Task{
			Name:         "Destroy ETCD-related custom resource definitions",
			Fn:           component.OpDestroyAndWait(c.etcdCRD).Destroy,
			Dependencies: flow.NewTaskIDs(destroyEtcdDruid),
			SkipIf:       seedIsGarden,
		})
		destroyFluentOperatorCRDs = g.Add(flow.Task{
			Name:         "Destroy Fluent Operator CRDs",
			Fn:           component.OpDestroyAndWait(c.fluentCRD).Destroy,
			Dependencies: flow.NewTaskIDs(destroyFluentOperatorResources, noControllerInstallations),
			SkipIf:       seedIsGarden,
		})

		syncPointCleanedUp = flow.NewTaskIDs(
			destroyGardenerCustomMetrics,
			destroyClusterIdentity,
			destroyCachePrometheus,
			destroySeedPrometheus,
			destroyAggregatePrometheus,
			destroyAlertManager,
			destroyNginxIngress,
			destroyClusterAutoscaler,
			destroyMachineControllerManager,
			destroyDWDWeeder,
			destroyDWDProber,
			destroyKubeAPIServerIngress,
			destroyKubeAPIServerService,
			destroyVPNAuthzServer,
			destroyIstio,
			destroyIstioCRDs,
			destroyMachineControllerManagerCRDs,
			destroyFluentOperatorResources,
			noControllerInstallations,
			destroyPrometheusOperator,
			destroyPlutono,
			destroyKubeStateMetrics,
			destroyEtcdDruid,
			destroyHVPA,
			destroyVPA,
			destroyFluentBit,
			destroyFluentOperator,
			destroyVali,
			destroyEtcdCRD,
			destroyFluentOperatorCRDs,
		)

		destroySystemResources = g.Add(flow.Task{
			Name:         "Destroy system resources",
			Fn:           component.OpDestroyAndWait(c.system).Destroy,
			Dependencies: flow.NewTaskIDs(syncPointCleanedUp),
		})
		ensureNoManagedResourcesExist = g.Add(flow.Task{
			Name:         "Ensuring all ManagedResources are gone",
			Fn:           ensureNoManagedResources(r.SeedClientSet.Client()),
			Dependencies: flow.NewTaskIDs(destroySystemResources),
			SkipIf:       seedIsGarden,
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying gardener-resource-manager",
			Fn:           c.gardenerResourceManager.Destroy,
			Dependencies: flow.NewTaskIDs(ensureNoManagedResourcesExist),
			SkipIf:       seedIsGarden,
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log:              log,
		ProgressReporter: r.reportProgress(log, seed.GetInfo()),
	}); err != nil {
		return flow.Errors(err)
	}

	return nil
}

func ensureNoControllerInstallations(c client.Client, seedName string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		associatedControllerInstallations, err := controllerutils.DetermineControllerInstallationAssociations(ctx, c, seedName)
		if err != nil {
			return err
		}

		if associatedControllerInstallations != nil {
			return fmt.Errorf("can't continue with Seed deletion, because the following objects are still referencing it: ControllerInstallations=%v", associatedControllerInstallations)
		}

		return nil
	}
}

func ensureNoManagedResources(c client.Client) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		managedResourcesStillExist, err := managedresources.CheckIfManagedResourcesExist(ctx, c, ptr.To(v1beta1constants.SeedResourceManagerClass))
		if err != nil {
			return err
		}
		if managedResourcesStillExist {
			return fmt.Errorf("at least one ManagedResource still exists, cannot delete gardener-resource-manager")
		}
		return nil
	}
}
