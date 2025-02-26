// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/extensions/dnsrecord"
	"github.com/gardener/gardener/pkg/component/garden/system/virtual"
	gardeneraccess "github.com/gardener/gardener/pkg/component/gardener/access"
	"github.com/gardener/gardener/pkg/component/gardener/resourcemanager"
	"github.com/gardener/gardener/pkg/component/kubernetes/controllermanager"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus"
	gardenprometheus "github.com/gardener/gardener/pkg/component/observability/monitoring/prometheus/garden"
	"github.com/gardener/gardener/pkg/controllerutils"
	reconcilerutils "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
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
	c, err := r.instantiateComponents(ctx, log, garden, secretsManager, targetVersion, kubernetes.NewApplier(r.RuntimeClientSet.Client(), r.RuntimeClientSet.Client().RESTMapper()), nil, false)
	if err != nil {
		return reconcile.Result{}, err
	}

	const (
		defaultTimeout  = 30 * time.Second
		defaultInterval = 5 * time.Second
	)

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
		destroyAlertmanager = g.Add(flow.Task{
			Name: "Destroying Alertmanager",
			Fn:   component.OpDestroyAndWait(c.alertManager).Destroy,
		})
		destroyPrometheusLongTerm = g.Add(flow.Task{
			Name: "Destroying long-term Prometheus",
			Fn:   component.OpDestroyAndWait(c.prometheusLongTerm).Destroy,
		})
		destroyPrometheusGarden = g.Add(flow.Task{
			Name: "Destroying Garden Prometheus",
			Fn: func(ctx context.Context) error {
				return r.destroyGardenPrometheus(ctx, c.prometheusGarden)
			},
		})
		destroyBlackboxExporter = g.Add(flow.Task{
			Name: "Destroying blackbox-exporter",
			Fn:   component.OpDestroyAndWait(c.blackboxExporter).Destroy,
		})
		destroyGardenerOperatorVPA = g.Add(flow.Task{
			Name: "Destroying VPA for gardener-operator",
			Fn: func(ctx context.Context) error {
				return gardenerutils.DeleteVPAForGardenerComponent(ctx, r.RuntimeClientSet.Client(), v1beta1constants.DeploymentNameGardenerOperator, r.GardenNamespace)
			},
		})

		destroyGardenerDiscoveryServer = g.Add(flow.Task{
			Name: "Destroying Gardener Discovery Server",
			Fn:   component.OpDestroyAndWait(c.gardenerDiscoveryServer).Destroy,
		})
		destroyTerminalControllerManager = g.Add(flow.Task{
			Name: "Destroying Gardener Dashboard web terminal controller manager",
			Fn:   component.OpDestroyAndWait(c.terminalControllerManager).Destroy,
		})
		destroyGardenerDashboard = g.Add(flow.Task{
			Name: "Destroying Gardener Dashboard",
			Fn:   component.OpDestroyAndWait(c.gardenerDashboard).Destroy,
		})
		destroyGardenerScheduler = g.Add(flow.Task{
			Name: "Destroying Gardener Scheduler",
			Fn:   component.OpDestroyAndWait(c.gardenerScheduler).Destroy,
		})
		deleteExtensionResourcesBeforeKubeAPIServer = g.Add(flow.Task{
			Name: "Destroying extension resources before kube-apiserver",
			Fn:   flow.TaskFn(c.extensions.DestroyBeforeKubeAPIServer).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		waitUntilExtensionResourcesBeforeKubeAPIServerDeleted = g.Add(flow.Task{
			Name:         "Waiting until extension resources that should be handled before kube-apiserver have been deleted",
			Fn:           c.extensions.WaitCleanupBeforeKubeAPIServer,
			Dependencies: flow.NewTaskIDs(deleteExtensionResourcesBeforeKubeAPIServer),
		})
		destroyGardenerControllerManager = g.Add(flow.Task{
			Name:         "Destroying Gardener Controller Manager",
			Fn:           component.OpDestroyAndWait(c.gardenerControllerManager).Destroy,
			Dependencies: flow.NewTaskIDs(waitUntilExtensionResourcesBeforeKubeAPIServerDeleted),
		})
		destroyGardenerAdmissionController = g.Add(flow.Task{
			Name:         "Destroying Gardener Admission Controller",
			Fn:           component.OpDestroyAndWait(c.gardenerAdmissionController).Destroy,
			Dependencies: flow.NewTaskIDs(waitUntilExtensionResourcesBeforeKubeAPIServerDeleted),
		})
		destroyGardenerAPIServer = g.Add(flow.Task{
			Name:         "Destroying Gardener API Server",
			Fn:           component.OpDestroyAndWait(c.gardenerAPIServer).Destroy,
			Dependencies: flow.NewTaskIDs(waitUntilExtensionResourcesBeforeKubeAPIServerDeleted),
		})
		destroyVirtualSystemResources = g.Add(flow.Task{
			Name:         "Destroying virtual system resources",
			Fn:           component.OpDestroyAndWait(c.virtualSystem).Destroy,
			Dependencies: flow.NewTaskIDs(destroyGardenerAPIServer),
		})

		syncPointVirtualGardenManagedResourcesDestroyed = flow.NewTaskIDs(
			destroyGardenerDiscoveryServer,
			destroyTerminalControllerManager,
			destroyGardenerDashboard,
			destroyGardenerScheduler,
			destroyGardenerControllerManager,
			destroyGardenerAdmissionController,
			destroyGardenerAPIServer,
			destroyVirtualSystemResources,
		)

		ensureOnlyCleanupRelevantVirtualManagedResourcesExist = g.Add(flow.Task{
			Name:         "Ensuring only virtual garden ManagedResources which are required for cleanup exist",
			Fn:           r.checkIfOnlyCleanupRelevantVirtualGardenManagedResourcesExist(),
			Dependencies: flow.NewTaskIDs(syncPointVirtualGardenManagedResourcesDestroyed),
		})

		destroyVirtualGardenGardenerAccess = g.Add(flow.Task{
			Name:         "Destroying Gardener virtual garden access resources",
			Fn:           component.OpDestroyAndWait(c.virtualGardenGardenerAccess).Destroy,
			Dependencies: flow.NewTaskIDs(ensureOnlyCleanupRelevantVirtualManagedResourcesExist),
		})
		destroyKubeControllerManager = g.Add(flow.Task{
			Name:         "Destroying Kubernetes Controller Manager Server",
			Fn:           component.OpDestroyAndWait(c.kubeControllerManager).Destroy,
			Dependencies: flow.NewTaskIDs(ensureOnlyCleanupRelevantVirtualManagedResourcesExist),
		})

		syncPointCleanupRelevantVirtualManagedResourcesDestroyed = flow.NewTaskIDs(
			destroyVirtualGardenGardenerAccess,
			destroyKubeControllerManager,
		)

		ensureNoVirtualGardenManagedResourcesExistAnymore = g.Add(flow.Task{
			Name:         "Ensuring no virtual garden ManagedResources exist anymore",
			Fn:           r.checkIfVirtualGardenManagedResourcesAreGone(),
			Dependencies: flow.NewTaskIDs(syncPointCleanupRelevantVirtualManagedResourcesDestroyed),
		})
		destroyVirtualGardenGardenerResourceManager = g.Add(flow.Task{
			Name:         "Destroying gardener-resource-manager for virtual garden",
			Fn:           component.OpDestroyAndWait(c.virtualGardenGardenerResourceManager).Destroy,
			Dependencies: flow.NewTaskIDs(syncPointVirtualGardenManagedResourcesDestroyed, ensureNoVirtualGardenManagedResourcesExistAnymore),
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
			Fn: func(_ context.Context) error {
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

		deleteExtensionResources = g.Add(flow.Task{
			Name: "Destroying extension resources",
			Fn:   flow.TaskFn(c.extensions.DeleteResources).RetryUntilTimeout(defaultInterval, defaultTimeout),
		})
		waitUntilExtensionResourcesDeleted = g.Add(flow.Task{
			Name:         "Waiting until extension resources have been deleted",
			Fn:           c.extensions.WaitCleanupResources,
			Dependencies: flow.NewTaskIDs(deleteExtensionResources),
		})
		destroyDNSRecords = g.Add(flow.Task{
			Name:         "Destroying DNSRecords for virtual garden cluster and ingress controller",
			Fn:           func(ctx context.Context) error { return r.destroyDNSRecords(ctx, log) },
			SkipIf:       garden.Spec.DNS == nil,
			Dependencies: flow.NewTaskIDs(syncPointVirtualGardenControlPlaneDestroyed),
		})
		destroyMainETCDBackupBucket = g.Add(flow.Task{
			Name: "Destroying main ETCD backup bucket",
			Fn: func(ctx context.Context) error {
				backupBucket := etcdMainBackupBucket(garden)
				if err := extensions.DeleteExtensionObject(ctx, r.RuntimeClientSet.Client(), backupBucket); err != nil {
					return err
				}

				return extensions.WaitUntilExtensionObjectDeleted(
					ctx,
					r.RuntimeClientSet.Client(),
					log,
					backupBucket,
					extensionsv1alpha1.BackupBucketResource,
					2*time.Second,
					time.Minute,
				)
			},
			SkipIf:       garden.Spec.VirtualCluster.ETCD == nil || garden.Spec.VirtualCluster.ETCD.Main == nil || garden.Spec.VirtualCluster.ETCD.Main.Backup == nil,
			Dependencies: flow.NewTaskIDs(syncPointVirtualGardenControlPlaneDestroyed),
		})
		destroyEtcdDruid = g.Add(flow.Task{
			Name:         "Destroying ETCD Druid",
			Fn:           component.OpDestroyAndWait(c.etcdDruid).Destroy,
			Dependencies: flow.NewTaskIDs(syncPointVirtualGardenControlPlaneDestroyed),
		})
		destroyIstio = g.Add(flow.Task{
			Name:         "Destroying Istio",
			Fn:           component.OpDestroyAndWait(c.istio).Destroy,
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
		destroyPrometheusOperator = g.Add(flow.Task{
			Name:         "Destroying prometheus-operator",
			Fn:           component.OpDestroyAndWait(c.prometheusOperator).Destroy,
			Dependencies: flow.NewTaskIDs(destroyAlertmanager, destroyPrometheusGarden, destroyPrometheusLongTerm),
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
			waitUntilExtensionResourcesDeleted,
			destroyDNSRecords,
			destroyMainETCDBackupBucket,
			destroyEtcdDruid,
			destroyIstio,
			destroyVerticalPodAutoscaler,
			destroyNginxIngressController,
			destroyFluentOperatorCustomResources,
			destroyFluentBit,
			destroyFluentOperator,
			destroyVali,
			destroyPrometheusOperator,
			destroyBlackboxExporter,
			destroyGardenerOperatorVPA,
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
			Name:         "Destroying custom resource definition for extensions",
			Fn:           c.extensionCRD.Destroy,
			Dependencies: flow.NewTaskIDs(destroyGardenerResourceManager),
		})
		_ = g.Add(flow.Task{
			Name:         "Destroying custom resource definition for prometheus-operator",
			Fn:           c.prometheusCRD.Destroy,
			Dependencies: flow.NewTaskIDs(destroyGardenerResourceManager),
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
			Fn:           c.vpaCRD.Destroy,
			SkipIf:       !vpaEnabled(garden.Spec.RuntimeCluster.Settings),
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
		_ = g.Add(flow.Task{
			Name:         "Cleaning up garbage-collectable ConfigMaps and Secrets",
			Fn:           r.cleanupGarbageCollectableResources,
			Dependencies: flow.NewTaskIDs(destroyGardenerResourceManager),
		})
	)

	gardenCopy := garden.DeepCopy()
	if err := g.Compile().Run(ctx, flow.Opts{
		Log:              log,
		ProgressReporter: r.reportProgress(log, gardenCopy, true),
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

func (r *Reconciler) checkIfOnlyCleanupRelevantVirtualGardenManagedResourcesExist() func(context.Context) error {
	return r.checkIfVirtualGardenManagedResourcesAreGone(virtual.ManagedResourceName, gardeneraccess.ManagedResourceName, controllermanager.ManagedResourceName)
}

func (r *Reconciler) checkIfVirtualGardenManagedResourcesAreGone(excludedNames ...string) func(context.Context) error {
	return func(ctx context.Context) error {
		managedResourcesStillExist, err := managedresources.CheckIfManagedResourcesExist(
			ctx,
			r.RuntimeClientSet.Client(),
			nil,
			append(excludedNames, resourcemanager.ManagedResourceName)...,
		)
		if err != nil {
			return err
		}

		if managedResourcesStillExist {
			return &reconcilerutils.RequeueAfterError{
				RequeueAfter: 5 * time.Second,
				Cause:        errors.New("at least one ManagedResource still exists"),
			}
		}

		return nil
	}
}

func (r *Reconciler) checkIfManagedResourcesExist() func(context.Context) error {
	return func(ctx context.Context) error {
		managedResourcesStillExist, err := managedresources.CheckIfManagedResourcesExist(
			ctx,
			r.RuntimeClientSet.Client(),
			ptr.To(v1beta1constants.SeedResourceManagerClass),
		)
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

func (r *Reconciler) destroyGardenPrometheus(ctx context.Context, prometheus prometheus.Interface) error {
	if err := component.OpDestroyAndWait(prometheus).Destroy(ctx); err != nil {
		return err
	}

	if err := kubernetesutils.DeleteObject(ctx, r.RuntimeClientSet.Client(), gardenerutils.NewShootAccessSecret(gardenprometheus.AccessSecretName, r.GardenNamespace).Secret); err != nil {
		return err
	}

	return r.RuntimeClientSet.Client().DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(r.GardenNamespace), client.MatchingLabels{v1beta1constants.GardenerPurpose: gardenerutils.LabelPurposeGlobalMonitoringSecret})
}

func (r *Reconciler) destroyDNSRecords(ctx context.Context, log logr.Logger) error {
	dnsRecordList := &extensionsv1alpha1.DNSRecordList{}
	if err := r.listManagedDNSRecords(ctx, dnsRecordList); err != nil {
		return fmt.Errorf("failed listing DNS records: %w", err)
	}

	var taskFns []flow.TaskFn

	for _, dnsRecord := range dnsRecordList.Items {
		taskFns = append(taskFns, func(ctx context.Context) error {
			return component.OpDestroyAndWait(dnsrecord.New(
				log,
				r.RuntimeClientSet.Client(),
				&dnsrecord.Values{
					Name:      dnsRecord.Name,
					Namespace: dnsRecord.Namespace,
				},
				dnsrecord.DefaultInterval,
				dnsrecord.DefaultSevereThreshold,
				dnsrecord.DefaultTimeout,
			)).Destroy(ctx)
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}

func (r *Reconciler) cleanupGarbageCollectableResources(ctx context.Context) error {
	matchingLabels := client.MatchingLabels{references.LabelKeyGarbageCollectable: references.LabelValueGarbageCollectable}

	if err := r.RuntimeClientSet.Client().DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace(r.GardenNamespace), matchingLabels); err != nil {
		return fmt.Errorf("failed to delete remaining garbage-collectable Secrets: %w", err)
	}
	if err := r.RuntimeClientSet.Client().DeleteAllOf(ctx, &corev1.ConfigMap{}, client.InNamespace(r.GardenNamespace), matchingLabels); err != nil {
		return fmt.Errorf("failed to delete remaining garbage-collectable ConfigMaps: %w", err)
	}

	return nil
}
