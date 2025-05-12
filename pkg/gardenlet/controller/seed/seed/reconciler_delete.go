// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/clusteridentity"
	"github.com/gardener/gardener/pkg/controllerutils"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

func (r *Reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	seedObj *seedpkg.Seed,
	seedIsGarden bool,
	seedIsShoot bool,
) (
	reconcile.Result,
	error,
) {
	seed := seedObj.GetInfo()

	if !sets.New(seed.Finalizers...).Has(gardencorev1beta1.GardenerName) {
		return reconcile.Result{}, nil
	}

	// Before deletion, it has to be ensured that no Shoots nor BackupBuckets depend on the Seed anymore.
	// When this happens the controller will remove the finalizers from the Seed so that it can be garbage collected.
	parentLogMessage := "Can't delete Seed, because the following objects are still referencing it:"

	associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(ctx, r.GardenClient, seed)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to determine shoots associated to this seed: %w", err)
	}

	if len(associatedShoots) > 0 {
		log.Info("Cannot delete Seed because the following Shoots are still referencing it", "shoots", associatedShoots)
		r.Recorder.Event(seed, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, fmt.Sprintf("%s Shoots=%v", parentLogMessage, associatedShoots))
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}

	if seed.Spec.Backup != nil {
		backupBucket := &gardencorev1beta1.BackupBucket{ObjectMeta: metav1.ObjectMeta{Name: string(seed.UID)}}

		if err := r.GardenClient.Delete(ctx, backupBucket); client.IgnoreNotFound(err) != nil {
			return reconcile.Result{}, fmt.Errorf("failed deleting backup bucket %s: %w", client.ObjectKeyFromObject(backupBucket), err)
		}
	}

	associatedBackupBuckets, err := controllerutils.DetermineBackupBucketAssociations(ctx, r.GardenClient, seed.Name)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to determine backup buckets associated to this seed: %w", err)
	}

	if len(associatedBackupBuckets) > 0 {
		log.Info("Cannot delete Seed because the following BackupBuckets are still referencing it", "backupBuckets", associatedBackupBuckets)
		r.Recorder.Event(seed, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, fmt.Sprintf("%s BackupBuckets=%v", parentLogMessage, associatedBackupBuckets))
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	log.Info("No Shoots or BackupBuckets are referencing the Seed, deletion accepted")
	if err := r.runDeleteSeedFlow(ctx, log, seedObj, seedIsGarden, seedIsShoot); err != nil {
		return reconcile.Result{}, err
	}

	// Remove finalizer from Seed
	if controllerutil.ContainsFinalizer(seed, gardencorev1beta1.GardenerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.GardenClient, seed, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) runDeleteSeedFlow(
	ctx context.Context,
	log logr.Logger,
	seed *seedpkg.Seed,
	seedIsGarden bool,
	seedIsShoot bool,
) error {
	log.Info("Instantiating component deployers")
	c, err := r.instantiateComponents(ctx, log, seed, nil, seedIsGarden, nil, nil, nil, seedIsShoot)
	if err != nil {
		return err
	}

	seedIsOriginOfClusterIdentity, err := clusteridentity.IsClusterIdentityEmptyOrFromOrigin(ctx, r.SeedClientSet.Client(), v1beta1constants.ClusterIdentityOriginSeed)
	if err != nil {
		return err
	}

	var (
		g = flow.NewGraph("Seed deletion")

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
		// TODO(Wieneo): Remove this after Gardener v1.117 was released
		destroyVPNAuthzServer = g.Add(flow.Task{
			Name: "Destroy VPN authorization server",
			Fn:   component.OpDestroyAndWait(c.vpnAuthzServer).Destroy,
		})
		destroyIstio = g.Add(flow.Task{
			Name: "Destroy Istio",
			Fn:   component.OpDestroyAndWait(c.istio).Destroy,
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
			Name:   "Destroying etcd druid",
			Fn:     component.OpDestroyAndWait(c.etcdDruid).Destroy,
			SkipIf: seedIsGarden,
		})
		destroyVPA = g.Add(flow.Task{
			Name:   "Destroy Kubernetes vertical pod autoscaler",
			Fn:     component.OpDestroyAndWait(c.verticalPodAutoscaler).Destroy,
			SkipIf: seedIsGarden,
		})
		destroyKubeStateMetrics = g.Add(flow.Task{
			Name: "Destroy kube-state-metrics",
			Fn:   component.OpDestroyAndWait(c.kubeStateMetrics).Destroy,
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
		destroyGardenletVPA = g.Add(flow.Task{
			Name: "Destroying VPA for gardenlet",
			Fn: func(ctx context.Context) error {
				return gardenerutils.DeleteVPAForGardenerComponent(ctx, r.SeedClientSet.Client(), v1beta1constants.DeploymentNameGardenlet, r.GardenNamespace)
			},
		})
		destroyExtensionResources = g.Add(flow.Task{
			Name: "Deleting extension resources",
			Fn:   c.extension.Destroy,
		})
		waitUntilExtensionResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until extension resources have been deleted",
			Fn:           c.extension.WaitCleanup,
			Dependencies: flow.NewTaskIDs(destroyExtensionResources),
		})

		syncPointCleanedUp = flow.NewTaskIDs(
			destroyDNSRecord,
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
			destroyFluentOperatorResources,
			destroyPrometheusOperator,
			destroyPlutono,
			destroyKubeStateMetrics,
			destroyEtcdDruid,
			destroyVPA,
			destroyFluentBit,
			destroyFluentOperator,
			destroyVali,
			destroyGardenletVPA,
			waitUntilExtensionResourcesDeleted,
		)

		ensureNoControllerInstallationsExist = g.Add(flow.Task{
			Name:         "Ensuring all ControllerInstallations are gone",
			Fn:           ensureNoControllerInstallations(r.GardenClient, seed.GetInfo().Name),
			Dependencies: flow.NewTaskIDs(syncPointCleanedUp),
		})
		_ = g.Add(flow.Task{
			Name: "Deleting referenced resources",
			Fn: func(ctx context.Context) error {
				return r.destroyReferencedResources(ctx, seed)
			},
			Dependencies: flow.NewTaskIDs(ensureNoControllerInstallationsExist),
		})

		destroyIstioCRDs = g.Add(flow.Task{
			Name:         "Destroy Istio CRDs",
			Fn:           component.OpDestroyAndWait(c.istioCRD).Destroy,
			SkipIf:       seedIsGarden,
			Dependencies: flow.NewTaskIDs(ensureNoControllerInstallationsExist),
		})
		destroyMachineControllerManagerCRDs = g.Add(flow.Task{
			Name:         "Destroy machine-controller-manager CRDs",
			Fn:           component.OpDestroyAndWait(c.machineCRD).Destroy,
			Dependencies: flow.NewTaskIDs(ensureNoControllerInstallationsExist),
		})
		destroyEtcdCRD = g.Add(flow.Task{
			Name:         "Destroy ETCD-related custom resource definitions",
			Fn:           component.OpDestroyAndWait(c.etcdCRD).Destroy,
			Dependencies: flow.NewTaskIDs(ensureNoControllerInstallationsExist),
			SkipIf:       seedIsGarden,
		})
		destroyFluentOperatorCRDs = g.Add(flow.Task{
			Name:         "Destroy Fluent Operator CRDs",
			Fn:           component.OpDestroyAndWait(c.fluentCRD).Destroy,
			Dependencies: flow.NewTaskIDs(ensureNoControllerInstallationsExist),
			SkipIf:       seedIsGarden,
		})

		destroyCRDs = flow.NewTaskIDs(
			destroyIstioCRDs,
			destroyMachineControllerManagerCRDs,
			destroyEtcdCRD,
			destroyFluentOperatorCRDs,
		)

		destroySystemResources = g.Add(flow.Task{
			Name:         "Destroy system resources",
			Fn:           component.OpDestroyAndWait(c.system).Destroy,
			Dependencies: flow.NewTaskIDs(destroyCRDs),
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

		if len(associatedControllerInstallations) > 0 {
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
