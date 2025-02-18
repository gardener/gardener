// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	istiov1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	podsecurityadmissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/clusteridentity"
	"github.com/gardener/gardener/pkg/component/networking/istio"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/tokenrequest"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

func (r *Reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	seedObj *seedpkg.Seed,
	seedIsGarden bool,
	isManagedSeed bool,
) error {
	seed := seedObj.GetInfo()

	if !controllerutil.ContainsFinalizer(seed, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.GardenClient, seed, gardencorev1beta1.GardenerName); err != nil {
			return err
		}
	}

	// Check whether the Kubernetes version of the Seed cluster fulfills the minimal requirements.
	if err := r.checkMinimumK8SVersion(r.SeedClientSet.Version()); err != nil {
		return err
	}

	if err := r.runReconcileSeedFlow(ctx, log, seedObj, seedIsGarden, isManagedSeed); err != nil {
		return err
	}

	if seed.Spec.Backup != nil {
		// This should be post updating the seed is available. Since, scheduler will then mostly use
		// same seed for deploying the backupBucket extension.
		if err := deployBackupBucketInGarden(ctx, r.GardenClient, seed); err != nil {
			return err
		}
	}

	return nil
}

func (r *Reconciler) checkMinimumK8SVersion(version string) error {
	const minKubernetesVersion = "1.25"

	seedVersionOK, err := versionutils.CompareVersions(version, ">=", minKubernetesVersion)
	if err != nil {
		return err
	}
	if !seedVersionOK {
		return fmt.Errorf("the Kubernetes version of the Seed cluster must be at least %s", minKubernetesVersion)
	}

	return nil
}

func (r *Reconciler) runReconcileSeedFlow(
	ctx context.Context,
	log logr.Logger,
	seed *seedpkg.Seed,
	seedIsGarden bool,
	isManagedSeed bool,
) error {
	// VPA is a prerequisite. If it's enabled then we deploy the CRD (and later also the related components) as part of
	// the flow. However, when it's disabled then we check whether it is indeed available (and fail, otherwise).
	if !vpaEnabled(seed.GetInfo().Spec.Settings) {
		if _, err := r.SeedClientSet.Client().RESTMapper().RESTMapping(schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"}); err != nil {
			return fmt.Errorf("VPA is required for seed cluster: %s", err)
		}
	}

	// create + label garden namespace
	gardenNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: r.GardenNamespace}}
	log.Info("Labeling and annotating namespace", "namespaceName", gardenNamespace.Name)
	if _, err := controllerutils.CreateOrGetAndMergePatch(ctx, r.SeedClientSet.Client(), gardenNamespace, func() error {
		metav1.SetMetaDataLabel(&gardenNamespace.ObjectMeta, "role", v1beta1constants.GardenNamespace)

		// When the seed is the garden cluster then this information is managed by gardener-operator.
		if !seedIsGarden {
			metav1.SetMetaDataLabel(&gardenNamespace.ObjectMeta, podsecurityadmissionapi.EnforceLevelLabel, string(podsecurityadmissionapi.LevelPrivileged))
			metav1.SetMetaDataLabel(&gardenNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")
			metav1.SetMetaDataAnnotation(&gardenNamespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(seed.GetInfo().Spec.Provider.Zones, ","))
		}
		return nil
	}); err != nil {
		return err
	}

	// label kube-system namespace
	namespaceKubeSystem := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: metav1.NamespaceSystem}}
	log.Info("Labeling namespace", "namespaceName", namespaceKubeSystem.Name)
	patch := client.MergeFrom(namespaceKubeSystem.DeepCopy())
	metav1.SetMetaDataLabel(&namespaceKubeSystem.ObjectMeta, "role", metav1.NamespaceSystem)
	if err := r.SeedClientSet.Client().Patch(ctx, namespaceKubeSystem, patch); err != nil {
		return err
	}

	secretsManager, err := secretsmanager.New(
		ctx,
		log.WithName("secretsmanager"),
		clock.RealClock{},
		r.SeedClientSet.Client(),
		r.GardenNamespace,
		v1beta1constants.SecretManagerIdentityGardenlet,
		secretsmanager.Config{CASecretAutoRotation: true},
	)
	if err != nil {
		return err
	}

	// Deploy dedicated CA certificate for seed cluster, auto-rotate it roughly once a month and drop the old CA 24 hours
	// after rotation.
	log.Info("Generating CA certificates for seed cluster")
	if _, err := secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:       v1beta1constants.SecretNameCASeed,
		CommonName: "kubernetes",
		CertType:   secretsutils.CACert,
		Validity:   ptr.To(30 * 24 * time.Hour),
	}, secretsmanager.Rotate(secretsmanager.KeepOld), secretsmanager.IgnoreOldSecretsAfter(24*time.Hour)); err != nil {
		return err
	}

	secrets, err := gardenerutils.ReadGardenSecrets(ctx, log, r.GardenClient, gardenerutils.ComputeGardenNamespace(seed.GetInfo().Name), true)
	if err != nil {
		return err
	}

	// replicate global monitoring secret (read from garden cluster) to the seed cluster's garden namespace
	globalMonitoringSecretGarden, ok := secrets[v1beta1constants.GardenRoleGlobalMonitoring]
	if !ok {
		return errors.New("global monitoring secret not found in seed namespace")
	}

	log.Info("Replicating global monitoring secret to garden namespace in seed", "secret", client.ObjectKeyFromObject(globalMonitoringSecretGarden))
	globalMonitoringSecretSeed, err := gardenerutils.ReplicateGlobalMonitoringSecret(ctx, r.SeedClientSet.Client(), "seed-", r.GardenNamespace, globalMonitoringSecretGarden)
	if err != nil {
		return err
	}

	var alertingSMTPSecret *corev1.Secret
	if secret, ok := secrets[v1beta1constants.GardenRoleAlerting]; ok && string(secret.Data["auth_type"]) == "smtp" {
		alertingSMTPSecret = secret
	}

	wildcardCertSecret, err := gardenerutils.GetWildcardCertificate(ctx, r.SeedClientSet.Client())
	if err != nil {
		return err
	}

	log.Info("Instantiating component deployers")
	c, err := r.instantiateComponents(ctx, log, seed, secretsManager, seedIsGarden, globalMonitoringSecretSeed, alertingSMTPSecret, wildcardCertSecret, isManagedSeed)
	if err != nil {
		return err
	}

	seedIsOriginOfClusterIdentity, err := clusteridentity.IsClusterIdentityEmptyOrFromOrigin(ctx, r.SeedClientSet.Client(), v1beta1constants.ClusterIdentityOriginSeed)
	if err != nil {
		return err
	}

	var (
		g = flow.NewGraph("Seed reconciliation")

		deployMachineCRD = g.Add(flow.Task{
			Name: "Deploying machine-related custom resource definitions",
			Fn:   c.machineCRD.Deploy,
		})
		deployExtensionCRD = g.Add(flow.Task{
			Name: "Deploying extensions-related custom resource definitions",
			Fn:   c.extensionCRD.Deploy,
		})
		deployEtcdCRD = g.Add(flow.Task{
			Name:   "Deploying ETCD-related custom resource definitions",
			Fn:     c.etcdCRD.Deploy,
			SkipIf: seedIsGarden,
		})
		deployIstioCRD = g.Add(flow.Task{
			Name:   "Deploying Istio-related custom resource definitions",
			Fn:     c.istioCRD.Deploy,
			SkipIf: seedIsGarden,
		})
		deployVPACRD = g.Add(flow.Task{
			Name:   "Deploying VPA-related custom resource definitions",
			Fn:     c.vpaCRD.Deploy,
			SkipIf: seedIsGarden || !vpaEnabled(seed.GetInfo().Spec.Settings),
		})
		deployFluentCRD = g.Add(flow.Task{
			Name:   "Deploying logging-related custom resource definitions",
			Fn:     c.fluentCRD.Deploy,
			SkipIf: seedIsGarden,
		})
		deployPrometheusCRD = g.Add(flow.Task{
			Name:   "Deploying monitoring-related custom resource definitions",
			Fn:     component.OpWait(c.prometheusCRD).Deploy,
			SkipIf: seedIsGarden,
		})
		syncPointCRDs = flow.NewTaskIDs(
			deployMachineCRD,
			deployExtensionCRD,
			deployEtcdCRD,
			deployIstioCRD,
			deployVPACRD,
			deployFluentCRD,
			deployPrometheusCRD,
		)

		_ = g.Add(flow.Task{
			Name: "Deploying VPA for gardenlet",
			Fn: func(ctx context.Context) error {
				return gardenerutils.ReconcileVPAForGardenerComponent(ctx, r.SeedClientSet.Client(), v1beta1constants.DeploymentNameGardenlet, r.GardenNamespace)
			},
			Dependencies: flow.NewTaskIDs(syncPointCRDs),
		})
		deployGardenerResourceManager = g.Add(flow.Task{
			Name:         "Deploying and waiting for gardener-resource-manager to be healthy",
			Fn:           component.OpWait(c.gardenerResourceManager).Deploy,
			Dependencies: flow.NewTaskIDs(syncPointCRDs),
			SkipIf:       seedIsGarden,
		})
		deploySystemResources = g.Add(flow.Task{
			Name:         "Deploying system resources",
			Fn:           c.system.Deploy,
			Dependencies: flow.NewTaskIDs(deployGardenerResourceManager),
		})
		waitUntilRequiredExtensionsReady = g.Add(flow.Task{
			Name: "Waiting until required extensions are ready",
			Fn: func(ctx context.Context) error {
				return retry.UntilTimeout(ctx, 5*time.Second, time.Minute, func(ctx context.Context) (done bool, err error) {
					if err := gardenerutils.RequiredExtensionsReady(ctx, r.GardenClient, seed.GetInfo().Name, gardenerutils.ComputeRequiredExtensionsForSeed(seed.GetInfo())); err != nil {
						return retry.MinorError(err)
					}

					return retry.Ok()
				})
			},
			Dependencies: flow.NewTaskIDs(deploySystemResources),
		})
		// Use the managed resource for cluster-identity only if there is no cluster-identity config map in kube-system namespace from a different origin than seed.
		// This prevents gardenlet from deleting the config map accidentally on seed deletion when it was created by a different party (gardener-apiserver or shoot).
		deployClusterIdentity = g.Add(flow.Task{
			Name:         "Deploying cluster-identity",
			Fn:           c.clusterIdentity.Deploy,
			Dependencies: flow.NewTaskIDs(waitUntilRequiredExtensionsReady),
			SkipIf:       !seedIsOriginOfClusterIdentity,
		})
		cleanupOrphanedExposureClassHandlers = g.Add(flow.Task{
			Name: "Cleaning up orphan ExposureClass handler resources",
			Fn: func(ctx context.Context) error {
				return cleanupOrphanExposureClassHandlerResources(ctx, log, r.SeedClientSet.Client(), r.Config.ExposureClassHandlers, seed.GetInfo().Spec.Provider.Zones)
			},
			Dependencies: flow.NewTaskIDs(waitUntilRequiredExtensionsReady),
		})
		syncPointReadyForSystemComponents = flow.NewTaskIDs(
			deployGardenerResourceManager,
			deployClusterIdentity,
			cleanupOrphanedExposureClassHandlers,
		)

		deployIstio = g.Add(flow.Task{
			Name:         "Deploying Istio",
			Fn:           c.istio.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
		})
		_ = g.Add(flow.Task{
			Name: "Waiting until istio LoadBalancer is ready and managed ingress DNS record is reconciled",
			Fn: func(ctx context.Context) error {
				ingressDNSRecord, err := r.deployNginxIngressAndWaitForIstioServiceAndGetDNSComponent(ctx, log, seed, c.nginxIngressController, seedIsGarden, c.istioDefaultNamespace)
				if err != nil {
					return err
				}
				return component.OpWait(ingressDNSRecord).Deploy(ctx)
			},
			Dependencies: flow.NewTaskIDs(deployIstio),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying cluster-autoscaler resources",
			Fn:           c.clusterAutoscaler.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying machine-controller-manager resources",
			Fn:           c.machineControllerManager.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying dependency-watchdog-weeder",
			Fn:           c.dwdWeeder.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying dependency-watchdog-prober",
			Fn:           c.dwdProber.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying VPN authorization server",
			Fn:           c.vpnAuthzServer.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
		})
		_ = g.Add(flow.Task{
			Name: "Renewing garden access secrets",
			Fn: func(ctx context.Context) error {
				// renew access secrets in all namespaces with the resources.gardener.cloud/class=garden label
				if err := tokenrequest.RenewAccessSecrets(ctx, r.SeedClientSet.Client(), client.MatchingLabels{resourcesv1alpha1.ResourceManagerClass: resourcesv1alpha1.ResourceManagerClassGarden}); err != nil {
					return err
				}

				// remove operation annotation from seed after successful operation
				return removeSeedOperationAnnotation(ctx, r.GardenClient, seed)
			},
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
			SkipIf:       seed.GetInfo().Annotations[v1beta1constants.GardenerOperation] != v1beta1constants.SeedOperationRenewGardenAccessSecrets,
		})

		_ = g.Add(flow.Task{
			Name: "Renewing workload identity tokens",
			Fn: func(ctx context.Context) error {
				// renew workload identity tokens in all namespaces with the security.gardener.cloud/purpose=workload-identity-token-requestor label
				if err := tokenrequest.RenewWorkloadIdentityTokens(ctx, r.SeedClientSet.Client()); err != nil {
					return err
				}

				// remove operation annotation from seed after successful operation
				return removeSeedOperationAnnotation(ctx, r.GardenClient, seed)
			},
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
			SkipIf:       seed.GetInfo().Annotations[v1beta1constants.GardenerOperation] != v1beta1constants.SeedOperationRenewWorkloadIdentityTokens,
		})

		_ = g.Add(flow.Task{
			Name: "Renewing garden kubeconfig",
			Fn: func(ctx context.Context) error {
				if err := renewGardenKubeconfig(ctx, r.SeedClientSet.Client(), r.Config.GardenClientConnection); err != nil {
					return err
				}

				// remove operation annotation from seed after successful operation
				return removeSeedOperationAnnotation(ctx, r.GardenClient, seed)
			},
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
			SkipIf:       seed.GetInfo().Annotations[v1beta1constants.GardenerOperation] != v1beta1constants.GardenerOperationRenewKubeconfig,
		})
		_ = g.Add(flow.Task{
			Name:         "Reconciling kube-apiserver service",
			Fn:           c.kubeAPIServerService.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
		})
		_ = g.Add(flow.Task{
			Name:         "Reconciling kube-apiserver ingress",
			Fn:           c.kubeAPIServerIngress.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
		})

		// When the seed is the garden cluster then the following components are reconciled by the gardener-operator.
		_ = g.Add(flow.Task{
			Name:         "Deploying Kubernetes vertical pod autoscaler",
			Fn:           c.verticalPodAutoscaler.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
			SkipIf:       seedIsGarden,
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying ETCD Druid",
			Fn:           c.etcdDruid.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
			SkipIf:       seedIsGarden,
		})

		_ = g.Add(flow.Task{
			Name:         "Deploying kube-state-metrics",
			Fn:           c.kubeStateMetrics.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
		})
		deployFluentOperator = g.Add(flow.Task{
			Name:         "Deploying Fluent Operator",
			Fn:           c.fluentOperator.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
			SkipIf:       seedIsGarden,
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Fluent Bit",
			Fn:           c.fluentBit.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents, deployFluentOperator),
			SkipIf:       seedIsGarden,
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Fluent Operator custom resources",
			Fn:           c.fluentOperatorCustomResources.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents, deployFluentOperator),
			SkipIf:       seedIsGarden,
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Plutono",
			Fn:           c.plutono.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
			SkipIf:       seedIsGarden,
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Vali",
			Fn:           c.vali.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
			SkipIf:       seedIsGarden,
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Prometheus Operator",
			Fn:           c.prometheusOperator.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
			SkipIf:       seedIsGarden,
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying cache Prometheus",
			Fn:           c.cachePrometheus.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying seed Prometheus",
			Fn:           c.seedPrometheus.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying aggregate Prometheus",
			Fn:           c.aggregatePrometheus.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
		})
		_ = g.Add(flow.Task{
			Name:         "Deploying Alertmanager",
			Fn:           c.alertManager.Deploy,
			Dependencies: flow.NewTaskIDs(syncPointReadyForSystemComponents),
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log:              log,
		ProgressReporter: r.reportProgress(log, seed.GetInfo()),
	}); err != nil {
		return flow.Errors(err)
	}

	return secretsManager.Cleanup(ctx)
}

func deployBackupBucketInGarden(ctx context.Context, k8sGardenClient client.Client, seed *gardencorev1beta1.Seed) error {
	// By default, we assume the seed.Spec.Backup.Provider matches the seed.Spec.Provider.Type as per the validation logic.
	// However, if the backup region is specified we take it.
	region := seed.Spec.Provider.Region
	if seed.Spec.Backup.Region != nil {
		region = *seed.Spec.Backup.Region
	}

	backupBucket := &gardencorev1beta1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: string(seed.UID),
		},
	}

	ownerRef := metav1.NewControllerRef(seed, gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"))

	_, err := controllerutils.CreateOrGetAndMergePatch(ctx, k8sGardenClient, backupBucket, func() error {
		metav1.SetMetaDataAnnotation(&backupBucket.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		backupBucket.OwnerReferences = []metav1.OwnerReference{*ownerRef}
		backupBucket.Spec = gardencorev1beta1.BackupBucketSpec{
			Provider: gardencorev1beta1.BackupBucketProvider{
				Type:   seed.Spec.Backup.Provider,
				Region: region,
			},
			ProviderConfig: seed.Spec.Backup.ProviderConfig,
			SecretRef: corev1.SecretReference{
				Name:      seed.Spec.Backup.SecretRef.Name,
				Namespace: seed.Spec.Backup.SecretRef.Namespace,
			},
			SeedName: &seed.Name, // In future this will be moved to gardener-scheduler.
		}
		return nil
	})
	return err
}

func cleanupOrphanExposureClassHandlerResources(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	exposureClassHandlers []gardenletconfigv1alpha1.ExposureClassHandler,
	zones []string,
) error {
	// Remove ordinary, orphaned istio exposure class namespaces
	exposureClassHandlerNamespaces := &corev1.NamespaceList{}
	if err := c.List(ctx, exposureClassHandlerNamespaces, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExposureClassHandler}); err != nil {
		return err
	}

	for _, namespace := range exposureClassHandlerNamespaces.Items {
		if err := cleanupOrphanIstioNamespace(ctx, log, c, namespace, true, func() bool {
			for _, handler := range exposureClassHandlers {
				if *handler.SNI.Ingress.Namespace == namespace.Name {
					return true
				}
			}
			return false
		}); err != nil {
			return err
		}
	}

	// Remove zonal, orphaned istio exposure class namespaces
	zonalExposureClassHandlerNamespaces := &corev1.NamespaceList{}
	if err := c.List(ctx, zonalExposureClassHandlerNamespaces, client.MatchingLabelsSelector{
		Selector: labels.NewSelector().Add(utils.MustNewRequirement(v1beta1constants.GardenRole, selection.Exists)).Add(utils.MustNewRequirement(v1beta1constants.LabelExposureClassHandlerName, selection.Exists)),
	}); err != nil {
		return err
	}

	zoneSet := sets.New(zones...)
	for _, namespace := range zonalExposureClassHandlerNamespaces.Items {
		if ok, zone := sharedcomponent.IsZonalIstioExtension(namespace.Labels); ok {
			if err := cleanupOrphanIstioNamespace(ctx, log, c, namespace, true, func() bool {
				if !zoneSet.Has(zone) {
					return false
				}
				for _, handler := range exposureClassHandlers {
					if handler.Name == namespace.Labels[v1beta1constants.LabelExposureClassHandlerName] {
						return true
					}
				}
				return false
			}); err != nil {
				return err
			}
		}
	}

	// Remove zonal, orphaned istio default namespaces
	zonalIstioNamespaces := &corev1.NamespaceList{}
	if err := c.List(ctx, zonalIstioNamespaces, client.MatchingLabelsSelector{
		Selector: labels.NewSelector().Add(utils.MustNewRequirement(istio.DefaultZoneKey, selection.Exists)),
	}); err != nil {
		return err
	}

	for _, namespace := range zonalIstioNamespaces.Items {
		if ok, zone := sharedcomponent.IsZonalIstioExtension(namespace.Labels); ok {
			if err := cleanupOrphanIstioNamespace(ctx, log, c, namespace, false, func() bool {
				return zoneSet.Has(zone)
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func cleanupOrphanIstioNamespace(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	namespace corev1.Namespace,
	needsHandler bool,
	isAliveFunc func() bool,
) error {
	log = log.WithValues("namespace", client.ObjectKeyFromObject(&namespace))

	if isAlive := isAliveFunc(); isAlive {
		return nil
	}
	log.Info("Namespace is orphan as there is no ExposureClass handler in the gardenlet configuration anymore or the zone was removed")

	// Determine the corresponding handler name to the ExposureClass handler resources.
	handlerName, ok := namespace.Labels[v1beta1constants.LabelExposureClassHandlerName]
	if !ok && needsHandler {
		log.Info("Cannot delete ExposureClass handler resources as the corresponding handler is unknown and it is not save to remove them")
		return nil
	}

	gatewayList := &istiov1beta1.GatewayList{}
	if err := c.List(ctx, gatewayList); err != nil {
		return err
	}

	for _, gateway := range gatewayList.Items {
		if gateway.Name != v1beta1constants.DeploymentNameKubeAPIServer && gateway.Name != v1beta1constants.DeploymentNameVPNSeedServer {
			continue
		}
		if needsHandler {
			// Check if the gateway still selects the ExposureClass handler ingress gateway.
			if value, ok := gateway.Spec.Selector[v1beta1constants.LabelExposureClassHandlerName]; ok && value == handlerName {
				log.Info("Resources of ExposureClass handler cannot be deleted as they are still in use", "exposureClassHandler", handlerName)
				return nil
			}
		} else {
			_, zone := sharedcomponent.IsZonalIstioExtension(namespace.Labels)
			if value, ok := gateway.Spec.Selector[istio.DefaultZoneKey]; ok && strings.HasSuffix(value, zone) {
				log.Info("Resources of default zonal istio handler cannot be deleted as they are still in use", "zone", zone)
				return nil
			}
		}
	}

	// ExposureClass handler is orphan and not used by any Shoots anymore
	// therefore it is save to clean it up.
	log.Info("Delete orphan ExposureClass handler namespace")
	if err := c.Delete(ctx, &namespace); client.IgnoreNotFound(err) != nil {
		return err
	}

	return nil
}

func removeSeedOperationAnnotation(ctx context.Context, gardenClient client.Client, seed *seedpkg.Seed) error {
	return seed.UpdateInfo(ctx, gardenClient, false, func(seedObj *gardencorev1beta1.Seed) error {
		delete(seedObj.Annotations, v1beta1constants.GardenerOperation)
		return nil
	})
}

func renewGardenKubeconfig(ctx context.Context, seedClient client.Client, gardenClientConnection *gardenletconfigv1alpha1.GardenClientConnection) error {
	if gardenClientConnection == nil || gardenClientConnection.KubeconfigSecret == nil {
		return fmt.Errorf(
			"unable to renew garden kubeconfig. No gardenClientConnection.kubeconfigSecret specified in configuration of gardenlet. Remove \"%s=%s\" annotation from seed to reconcile successfully",
			v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRenewKubeconfig,
		)
	}

	kubeconfigSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: gardenClientConnection.KubeconfigSecret.Name, Namespace: gardenClientConnection.KubeconfigSecret.Namespace}}
	if err := seedClient.Get(ctx, client.ObjectKeyFromObject(kubeconfigSecret), kubeconfigSecret); err != nil {
		return err
	}

	return kubernetesutils.SetAnnotationAndUpdate(ctx, seedClient, kubeconfigSecret, v1beta1constants.GardenerOperation, v1beta1constants.KubeconfigSecretOperationRenew)
}

// WaitUntilLoadBalancerIsReady is an alias for kubernetesutils.WaitUntilLoadBalancerIsReady. Exposed for tests.
var WaitUntilLoadBalancerIsReady = kubernetesutils.WaitUntilLoadBalancerIsReady

func (r *Reconciler) deployNginxIngressAndWaitForIstioServiceAndGetDNSComponent(
	ctx context.Context,
	log logr.Logger,
	seed *seedpkg.Seed,
	nginxIngress component.DeployWaiter,
	seedIsGarden bool,
	istioDefaultNamespace string,
) (
	component.DeployWaiter,
	error,
) {
	if !seedIsGarden {
		if err := component.OpWait(nginxIngress).Deploy(ctx); err != nil {
			return nil, err
		}
	}

	ingressLoadBalancerAddress, err := WaitUntilLoadBalancerIsReady(
		ctx,
		log,
		r.SeedClientSet.Client(),
		istioDefaultNamespace,
		v1beta1constants.DefaultSNIIngressServiceName,
		time.Minute,
	)
	if err != nil {
		return nil, err
	}

	return r.newIngressDNSRecord(ctx, log, seed, ingressLoadBalancerAddress)
}
