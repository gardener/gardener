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

	"github.com/Masterminds/semver/v3"
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
	"github.com/gardener/gardener/pkg/component/clusterautoscaler"
	"github.com/gardener/gardener/pkg/component/clusteridentity"
	"github.com/gardener/gardener/pkg/component/dependencywatchdog"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/hvpa"
	"github.com/gardener/gardener/pkg/component/istio"
	"github.com/gardener/gardener/pkg/component/kubeapiserverexposure"
	"github.com/gardener/gardener/pkg/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/component/logging/fluentoperator"
	"github.com/gardener/gardener/pkg/component/logging/vali"
	"github.com/gardener/gardener/pkg/component/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/component/monitoring/prometheusoperator"
	"github.com/gardener/gardener/pkg/component/nginxingress"
	"github.com/gardener/gardener/pkg/component/plutono"
	"github.com/gardener/gardener/pkg/component/resourcemanager"
	"github.com/gardener/gardener/pkg/component/seedsystem"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/component/vpa"
	"github.com/gardener/gardener/pkg/component/vpnauthzserver"
	"github.com/gardener/gardener/pkg/controllerutils"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
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
	seedClient := r.SeedClientSet.Client()
	kubernetesVersion, err := semver.NewVersion(r.SeedClientSet.Version())
	if err != nil {
		return err
	}

	seedIsOriginOfClusterIdentity, err := clusteridentity.IsClusterIdentityEmptyOrFromOrigin(ctx, seedClient, v1beta1constants.ClusterIdentityOriginSeed)
	if err != nil {
		return err
	}

	secretData, err := getDNSProviderSecretData(ctx, r.GardenClient, seed.GetInfo())
	if err != nil {
		return err
	}

	istioIngressGateway := []istio.IngressGatewayValues{{Namespace: *r.Config.SNI.Ingress.Namespace}}
	if len(seed.GetInfo().Spec.Provider.Zones) > 1 {
		for _, zone := range seed.GetInfo().Spec.Provider.Zones {
			istioIngressGateway = append(istioIngressGateway, istio.IngressGatewayValues{Namespace: sharedcomponent.GetIstioNamespaceForZone(*r.Config.SNI.Ingress.Namespace, zone)})
		}
	}
	// Add for each ExposureClass handler in the config an own Ingress Gateway.
	for _, handler := range r.Config.ExposureClassHandlers {
		istioIngressGateway = append(istioIngressGateway, istio.IngressGatewayValues{Namespace: *handler.SNI.Ingress.Namespace})
		if len(seed.GetInfo().Spec.Provider.Zones) > 1 {
			for _, zone := range seed.GetInfo().Spec.Provider.Zones {
				istioIngressGateway = append(istioIngressGateway, istio.IngressGatewayValues{Namespace: sharedcomponent.GetIstioNamespaceForZone(*handler.SNI.Ingress.Namespace, zone)})
			}
		}
	}

	// Delete all ingress objects in garden namespace which are not created as part of ManagedResources. This can be
	// removed once all seed system components are deployed as part of ManagedResources.
	// See https://github.com/gardener/gardener/issues/6062 for details.
	if err := seedClient.DeleteAllOf(ctx, &networkingv1.Ingress{}, client.InNamespace(r.GardenNamespace)); err != nil {
		return err
	}

	// setup for flow graph
	var (
		dnsRecord                = getManagedIngressDNSRecord(log, seedClient, r.GardenNamespace, seed.GetInfo().Spec.DNS, secretData, seed.GetIngressFQDN("*"), "")
		clusterAutoscaler        = clusterautoscaler.NewBootstrapper(seedClient, r.GardenNamespace)
		machineControllerManager = machinecontrollermanager.NewBootstrapper(seedClient, r.GardenNamespace)
		kubeAPIServerIngress     = kubeapiserverexposure.NewIngress(seedClient, r.GardenNamespace, kubeapiserverexposure.IngressValues{})
		kubeAPIServerService     = kubeapiserverexposure.NewInternalNameService(seedClient, r.GardenNamespace)
		nginxIngress             = nginxingress.New(seedClient, r.GardenNamespace, nginxingress.Values{ClusterType: component.ClusterTypeSeed})
		dwdWeeder                = dependencywatchdog.NewBootstrapper(seedClient, r.GardenNamespace, dependencywatchdog.BootstrapperValues{Role: dependencywatchdog.RoleWeeder})
		dwdProber                = dependencywatchdog.NewBootstrapper(seedClient, r.GardenNamespace, dependencywatchdog.BootstrapperValues{Role: dependencywatchdog.RoleProber})
		systemResources          = seedsystem.New(seedClient, r.GardenNamespace, seedsystem.Values{})
		vpnAuthzServer           = vpnauthzserver.New(seedClient, r.GardenNamespace, "", kubernetesVersion)
		istioCRDs                = istio.NewCRD(r.SeedClientSet.ChartApplier())
		istio                    = istio.NewIstio(seedClient, r.SeedClientSet.ChartRenderer(), istio.Values{
			Istiod: istio.IstiodValues{
				Enabled:   !seedIsGarden,
				Namespace: v1beta1constants.IstioSystemNamespace,
			},
			IngressGateway: istioIngressGateway,
		})
		mcmCRDs                       = machinecontrollermanager.NewCRD(r.SeedClientSet.Client(), r.SeedClientSet.Applier())
		fluentOperatorCustomResources = fluentoperator.NewCustomResources(seedClient, r.GardenNamespace, fluentoperator.CustomResourcesValues{})
	)

	cachePrometheus, err := defaultCachePrometheus(log, seedClient, r.GardenNamespace, seed)
	if err != nil {
		return err
	}

	var (
		g                = flow.NewGraph("Seed cluster deletion")
		destroyDNSRecord = g.Add(flow.Task{
			Name: "Destroying managed ingress DNS record (if existing)",
			Fn:   func(ctx context.Context) error { return destroyDNSResources(ctx, dnsRecord) },
		})
		noControllerInstallations = g.Add(flow.Task{
			Name:         "Ensuring no ControllerInstallations are left",
			Fn:           ensureNoControllerInstallations(r.GardenClient, seed.GetInfo().Name),
			Dependencies: flow.NewTaskIDs(destroyDNSRecord),
		})
		destroyCachePrometheus = g.Add(flow.Task{
			Name: "Destroying cache Prometheus",
			Fn:   cachePrometheus.Destroy,
		})
		destroyClusterAutoscaler = g.Add(flow.Task{
			Name: "Destroying cluster-autoscaler resources",
			Fn:   component.OpDestroyAndWait(clusterAutoscaler).Destroy,
		})
		destroyMachineControllerManager = g.Add(flow.Task{
			Name: "Destroying machine-controller-manager resources",
			Fn:   component.OpDestroyAndWait(machineControllerManager).Destroy,
		})
		destroyNginxIngress = g.Add(flow.Task{
			Name:   "Destroying nginx-ingress",
			Fn:     component.OpDestroyAndWait(nginxIngress).Destroy,
			SkipIf: seedIsGarden,
		})
		destroyDWDWeeder = g.Add(flow.Task{
			Name: "Destroy dependency-watchdog-weeder",
			Fn:   component.OpDestroyAndWait(dwdWeeder).Destroy,
		})
		destroyDWDProber = g.Add(flow.Task{
			Name: "Destroy dependency-watchdog-prober",
			Fn:   component.OpDestroyAndWait(dwdProber).Destroy,
		})
		destroyKubeAPIServerIngress = g.Add(flow.Task{
			Name: "Destroy kube-apiserver ingress",
			Fn:   component.OpDestroyAndWait(kubeAPIServerIngress).Destroy,
		})
		destroyKubeAPIServerService = g.Add(flow.Task{
			Name: "Destroy kube-apiserver service",
			Fn:   component.OpDestroyAndWait(kubeAPIServerService).Destroy,
		})
		destroyVPNAuthzServer = g.Add(flow.Task{
			Name: "Destroy VPN authorization server",
			Fn:   component.OpDestroyAndWait(vpnAuthzServer).Destroy,
		})
		destroyIstio = g.Add(flow.Task{
			Name: "Destroy Istio",
			Fn:   component.OpDestroyAndWait(istio).Destroy,
		})
		destroyIstioCRDs = g.Add(flow.Task{
			Name:         "Destroy Istio CRDs",
			Fn:           component.OpDestroyAndWait(istioCRDs).Destroy,
			SkipIf:       seedIsGarden,
			Dependencies: flow.NewTaskIDs(destroyIstio),
		})
		destroyMachineControllerManagerCRDs = g.Add(flow.Task{
			Name: "Destroy machine-controller-manager CRDs",
			Fn:   component.OpDestroyAndWait(mcmCRDs).Destroy,
		})
		destroyFluentOperatorResources = g.Add(flow.Task{
			Name: "Destroy Fluent Operator Custom Resources",
			Fn:   component.OpDestroyAndWait(fluentOperatorCustomResources).Destroy,
		})
		syncPointCleanedUp = flow.NewTaskIDs(
			destroyCachePrometheus,
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
		)
		destroySystemResources = g.Add(flow.Task{
			Name:         "Destroy system resources",
			Fn:           component.OpDestroyAndWait(systemResources).Destroy,
			Dependencies: flow.NewTaskIDs(syncPointCleanedUp),
		})
	)

	// Use the managed resource for cluster-identity only if there is no cluster-identity config map in kube-system namespace from a different origin than seed.
	// This prevents gardenlet from deleting the config map accidentally on seed deletion when it was created by a different party (gardener-apiserver or shoot).
	if seedIsOriginOfClusterIdentity {
		var (
			clusterIdentity = clusteridentity.NewForSeed(seedClient, r.GardenNamespace, "")

			destroyClusterIdentity = g.Add(flow.Task{
				Name: "Destroying cluster-identity",
				Fn:   component.OpDestroyAndWait(clusterIdentity).Destroy,
			})
		)
		syncPointCleanedUp.Insert(destroyClusterIdentity)
	}

	// When the seed is the garden cluster then these components are reconciled by the gardener-operator.
	if !seedIsGarden {
		var (
			plutono               = plutono.New(seedClient, r.GardenNamespace, nil, plutono.Values{})
			kubeStateMetrics      = kubestatemetrics.New(seedClient, r.GardenNamespace, nil, kubestatemetrics.Values{ClusterType: component.ClusterTypeSeed, KubernetesVersion: kubernetesVersion})
			etcdCRD               = etcd.NewCRD(seedClient, r.SeedClientSet.Applier())
			etcdDruid             = etcd.NewBootstrapper(seedClient, r.GardenNamespace, nil, r.Config.ETCDConfig, "", nil, "")
			hvpa                  = hvpa.New(seedClient, r.GardenNamespace, hvpa.Values{})
			verticalPodAutoscaler = vpa.New(seedClient, r.GardenNamespace, nil, vpa.Values{ClusterType: component.ClusterTypeSeed, RuntimeKubernetesVersion: kubernetesVersion})
			resourceManager       = resourcemanager.New(seedClient, r.GardenNamespace, nil, resourcemanager.Values{RuntimeKubernetesVersion: kubernetesVersion})
			fluentOperatorCRDs    = fluentoperator.NewCRDs(r.SeedClientSet.Applier())
			fluentOperator        = fluentoperator.NewFluentOperator(seedClient, r.GardenNamespace, fluentoperator.Values{})
			fluentBit             = fluentoperator.NewFluentBit(seedClient, r.GardenNamespace, fluentoperator.FluentBitValues{})
			vali                  = vali.New(seedClient, r.GardenNamespace, nil, vali.Values{})
			prometheusOperator    = prometheusoperator.New(seedClient, r.GardenNamespace, prometheusoperator.Values{})

			destroyPlutono = g.Add(flow.Task{
				Name: "Destroying plutono",
				Fn:   component.OpDestroyAndWait(plutono).Destroy,
			})
			destroyKubeStateMetrics = g.Add(flow.Task{
				Name: "Destroy kube-state-metrics",
				Fn:   component.OpDestroyAndWait(kubeStateMetrics).Destroy,
			})
			destroyEtcdDruid = g.Add(flow.Task{
				Name: "Destroying etcd druid",
				Fn:   component.OpDestroyAndWait(etcdDruid).Destroy,
				// only destroy Etcd CRD once all extension controllers are gone, otherwise they might not be able to start up
				// again (e.g. after being evicted by VPA)
				// see https://github.com/gardener/gardener/issues/6487#issuecomment-1220597217
				Dependencies: flow.NewTaskIDs(noControllerInstallations),
			})
			destroyVPA = g.Add(flow.Task{
				Name: "Destroy Kubernetes vertical pod autoscaler",
				Fn:   component.OpDestroyAndWait(verticalPodAutoscaler).Destroy,
			})
			destroyHVPA = g.Add(flow.Task{
				Name: "Destroy HVPA controller",
				Fn:   component.OpDestroyAndWait(hvpa).Destroy,
			})
			destroyPrometheusOperator = g.Add(flow.Task{
				Name: "Destroy Prometheus Operator",
				Fn:   component.OpDestroyAndWait(prometheusOperator).Destroy,
			})
			destroyFluentBit = g.Add(flow.Task{
				Name: "Destroy Fluent Bit",
				Fn:   component.OpDestroyAndWait(fluentBit).Destroy,
			})
			destroyFluentOperator = g.Add(flow.Task{
				Name:         "Destroy Fluent Operator",
				Fn:           component.OpDestroyAndWait(fluentOperator).Destroy,
				Dependencies: flow.NewTaskIDs(destroyFluentOperatorResources, destroyFluentBit),
			})
			destroyVali = g.Add(flow.Task{
				Name:         "Destroy Vali",
				Fn:           component.OpDestroyAndWait(vali).Destroy,
				Dependencies: flow.NewTaskIDs(destroyFluentOperatorResources),
			})
			destroyEtcdCRD = g.Add(flow.Task{
				Name:         "Destroy ETCD-related custom resource definitions",
				Fn:           component.OpDestroyAndWait(etcdCRD).Destroy,
				Dependencies: flow.NewTaskIDs(destroyEtcdDruid),
			})
			destroyFluentOperatorCRDs = g.Add(flow.Task{
				Name:         "Destroy Fluent Operator CRDs",
				Fn:           component.OpDestroyAndWait(fluentOperatorCRDs).Destroy,
				Dependencies: flow.NewTaskIDs(destroyFluentOperatorResources, noControllerInstallations),
			})
		)

		syncPointCleanedUp.Insert(
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

		var (
			ensureNoManagedResourcesExist = g.Add(flow.Task{
				Name: "Ensuring all ManagedResources are gone",
				Fn: func(ctx context.Context) error {
					managedResourcesStillExist, err := managedresources.CheckIfManagedResourcesExist(ctx, r.SeedClientSet.Client(), ptr.To(v1beta1constants.SeedResourceManagerClass))
					if err != nil {
						return err
					}
					if managedResourcesStillExist {
						return fmt.Errorf("at least one ManagedResource still exists, cannot delete gardener-resource-manager")
					}
					return nil
				},
				Dependencies: flow.NewTaskIDs(destroySystemResources),
			})
			_ = g.Add(flow.Task{
				Name:         "Destroying gardener-resource-manager",
				Fn:           resourceManager.Destroy,
				Dependencies: flow.NewTaskIDs(ensureNoManagedResourcesExist),
			})
		)
	}

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
