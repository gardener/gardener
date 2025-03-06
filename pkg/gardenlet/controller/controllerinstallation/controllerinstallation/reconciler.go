// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerinstallation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/component-base/featuregate"
	podsecurityadmissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	ctrlinstutils "github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/utils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	netutils "github.com/gardener/gardener/pkg/utils/net"
	"github.com/gardener/gardener/pkg/utils/oci"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

const finalizerName = "core.gardener.cloud/controllerinstallation"

const usablePortsRangeSize = 5

// RequeueDurationWhenResourceDeletionStillPresent is the duration used for requeuing when owned resources are still in
// the process of being deleted when deleting a ControllerInstallation.
var RequeueDurationWhenResourceDeletionStillPresent = 5 * time.Second

// Reconciler reconciles ControllerInstallations and deploys them into the seed cluster or the autonomous shoot cluster.
type Reconciler struct {
	GardenClient          client.Client
	GardenConfig          *rest.Config
	SeedClientSet         kubernetes.Interface
	HelmRegistry          oci.Interface
	Config                gardenletconfigv1alpha1.GardenletConfiguration
	Clock                 clock.Clock
	Identity              *gardencorev1beta1.Gardener
	GardenClusterIdentity string
	// BootstrapControlPlaneNode controls whether the pods are used to bootstrap a control plane node. If set to true,
	// they are deployed to the host network. In addition, all taints are tolerated to make sure they can get deployed
	// to nodes even when they are not ready yet. Furthermore, the replicas are set to 1 and a usable port range is
	// provided.
	BootstrapControlPlaneNode bool
}

// Reconcile reconciles ControllerInstallations and deploys them into the seed cluster or the autonomous shoot cluster.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	gardenCtx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	seedCtx, cancel := controllerutils.GetChildReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	controllerInstallation := &gardencorev1beta1.ControllerInstallation{}
	if err := r.GardenClient.Get(gardenCtx, request.NamespacedName, controllerInstallation); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if controllerInstallation.DeletionTimestamp != nil {
		return r.delete(gardenCtx, seedCtx, log, controllerInstallation)
	}
	return r.reconcile(gardenCtx, seedCtx, log, controllerInstallation)
}

func (r *Reconciler) reconcile(
	gardenCtx context.Context,
	seedCtx context.Context,
	log logr.Logger,
	controllerInstallation *gardencorev1beta1.ControllerInstallation,
) (
	reconcile.Result,
	error,
) {
	if !controllerutil.ContainsFinalizer(controllerInstallation, finalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(gardenCtx, r.GardenClient, controllerInstallation, finalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	var (
		conditionValid     = v1beta1helper.GetOrInitConditionWithClock(r.Clock, controllerInstallation.Status.Conditions, gardencorev1beta1.ControllerInstallationValid)
		conditionInstalled = v1beta1helper.GetOrInitConditionWithClock(r.Clock, controllerInstallation.Status.Conditions, gardencorev1beta1.ControllerInstallationInstalled)
	)

	defer func() {
		if err := patchConditions(gardenCtx, r.GardenClient, controllerInstallation, conditionValid, conditionInstalled); err != nil {
			log.Error(err, "Failed to patch conditions")
		}
	}()

	controllerRegistration := &gardencorev1beta1.ControllerRegistration{}
	if err := r.GardenClient.Get(gardenCtx, client.ObjectKey{Name: controllerInstallation.Spec.RegistrationRef.Name}, controllerRegistration); err != nil {
		if apierrors.IsNotFound(err) {
			conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionFalse, "RegistrationNotFound", fmt.Sprintf("Referenced ControllerRegistration does not exist: %+v", err))
		} else {
			conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionUnknown, "RegistrationReadError", fmt.Sprintf("Referenced ControllerRegistration cannot be read: %+v", err))
		}
		return reconcile.Result{}, err
	}

	seed := &gardencorev1beta1.Seed{}
	if err := r.GardenClient.Get(gardenCtx, client.ObjectKey{Name: controllerInstallation.Spec.SeedRef.Name}, seed); err != nil {
		if apierrors.IsNotFound(err) {
			conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionFalse, "SeedNotFound", fmt.Sprintf("Referenced Seed does not exist: %+v", err))
		} else {
			conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionUnknown, "SeedReadError", fmt.Sprintf("Referenced Seed cannot be read: %+v", err))
		}
		return reconcile.Result{}, err
	}

	controllerDeployment := &gardencorev1.ControllerDeployment{}
	if deploymentRef := controllerInstallation.Spec.DeploymentRef; deploymentRef != nil {
		if err := r.GardenClient.Get(gardenCtx, client.ObjectKey{Name: deploymentRef.Name}, controllerDeployment); err != nil {
			return reconcile.Result{}, err
		}
		if controllerDeployment.Helm == nil {
			return reconcile.Result{}, nil
		}
	}

	var helmValues map[string]interface{}
	if controllerDeployment.Helm != nil && controllerDeployment.Helm.Values != nil {
		if err := json.Unmarshal(controllerDeployment.Helm.Values.Raw, &helmValues); err != nil {
			conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionFalse, "ChartInformationInvalid", fmt.Sprintf("chart values cannot be unmarshalled: %+v", err))
			return reconcile.Result{}, err
		}
	}

	seedIsGarden, err := gardenletutils.SeedIsGarden(seedCtx, r.SeedClientSet.Client())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed checking whether the seed is the garden cluster at the same time: %w", err)
	}

	namespace := getNamespaceForControllerInstallation(controllerInstallation)
	if _, err := controllerutils.GetAndCreateOrMergePatch(seedCtx, r.SeedClientSet.Client(), namespace, func() error {
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.GardenRole, v1beta1constants.GardenRoleExtension)
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.LabelControllerRegistrationName, controllerRegistration.Name)
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")
		metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(seed.Spec.Provider.Zones, ","))

		if seedIsGarden {
			metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.LabelNetworkPolicyAccessTargetAPIServer, "allowed")
		}

		if podSecurityEnforce, ok := controllerInstallation.Annotations[v1beta1constants.AnnotationPodSecurityEnforce]; ok {
			metav1.SetMetaDataLabel(&namespace.ObjectMeta, podsecurityadmissionapi.EnforceLevelLabel, podSecurityEnforce)
		} else {
			delete(namespace.Labels, podsecurityadmissionapi.EnforceLevelLabel)
		}

		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	if seed.Status.ClusterIdentity == nil {
		return reconcile.Result{}, fmt.Errorf("cluster-identity of seed '%s' not set", seed.Name)
	}

	var (
		injectGardenKubeconfig            = ptr.Deref(controllerDeployment.InjectGardenKubeconfig, false)
		genericGardenKubeconfigSecretName string
	)

	if injectGardenKubeconfig {
		genericGardenKubeconfigSecretName, err = r.reconcileGenericGardenKubeconfig(seedCtx, namespace.Name)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to reconcile generic garden kubeconfig: %w", err)
		}
	}

	var (
		gardenAccessSecret = gardenerutils.NewGardenAccessSecret("extension", namespace.Name).
			WithServiceAccountName(v1beta1constants.ExtensionGardenServiceAccountPrefix + controllerInstallation.Name).
			WithServiceAccountLabels(map[string]string{v1beta1constants.LabelControllerRegistrationName: controllerRegistration.Name})

		volumeProvider  string
		volumeProviders []gardencorev1beta1.SeedVolumeProvider
	)

	if seed.Spec.Volume != nil {
		volumeProviders = seed.Spec.Volume.Providers
		if len(seed.Spec.Volume.Providers) > 0 {
			volumeProvider = seed.Spec.Volume.Providers[0].Name
		}
	}

	featureToEnabled := make(map[featuregate.Feature]bool)
	for feature := range features.DefaultFeatureGate.GetAll() {
		featureToEnabled[feature] = features.DefaultFeatureGate.Enabled(feature)
	}

	// Mix-in some standard values for garden and seed.
	gardenerValues := map[string]any{
		"gardener": map[string]any{
			"version": r.Identity.Version,
			"garden": map[string]any{
				"clusterIdentity": r.GardenClusterIdentity,
			},
			"seed": map[string]any{
				"name":            seed.Name,
				"clusterIdentity": *seed.Status.ClusterIdentity,
				"annotations":     seed.Annotations,
				"labels":          seed.Labels,
				"provider":        seed.Spec.Provider.Type,
				"region":          seed.Spec.Provider.Region,
				"volumeProvider":  volumeProvider,
				"volumeProviders": volumeProviders,
				"ingressDomain":   &seed.Spec.Ingress.Domain,
				"protected":       v1beta1helper.TaintsHave(seed.Spec.Taints, gardencorev1beta1.SeedTaintProtected),
				"visible":         seed.Spec.Settings.Scheduling.Visible,
				"taints":          seed.Spec.Taints,
				"networks":        seed.Spec.Networks,
				"blockCIDRs":      seed.Spec.Networks.BlockCIDRs,
				"spec":            seed.Spec,
			},
			"gardenlet": map[string]any{
				"featureGates": featureToEnabled,
			},
		},
	}

	if genericGardenKubeconfigSecretName != "" {
		gardenerValues["gardener"].(map[string]any)["garden"].(map[string]any)["genericKubeconfigSecretName"] = genericGardenKubeconfigSecretName
	}

	if r.BootstrapControlPlaneNode {
		ports, err := r.CalculateUsablePorts()
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to calculate usable ports: %w", err)
		}
		gardenerValues["usablePorts"] = ports
	}

	archive := controllerDeployment.Helm.RawChart
	if len(archive) == 0 {
		var err error
		archive, err = r.HelmRegistry.Pull(seedCtx, controllerDeployment.Helm.OCIRepository)
		if err != nil {
			conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionFalse, "OCIChartCannotBePulled", fmt.Sprintf("chart pulling process failed: %+v", err))
			return reconcile.Result{}, err
		}
	}

	release, err := r.SeedClientSet.ChartRenderer().RenderArchive(archive, controllerRegistration.Name, namespace.Name, utils.MergeMaps(helmValues, gardenerValues))
	if err != nil {
		conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionFalse, "ChartCannotBeRendered", fmt.Sprintf("chart rendering process failed: %+v", err))
		return reconcile.Result{}, err
	}
	conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionTrue, "RegistrationValid", "chart could be rendered successfully.")
	secretData := release.AsSecretData()

	mustInjectGardenKubeconfig, err := needsGardenKubeconfig(injectGardenKubeconfig, secretData)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to check whether garden kubeconfig injection is needed: %w", err)
	}

	if mustInjectGardenKubeconfig {
		log.Info("Reconciling garden access secret")
		if err := gardenAccessSecret.Reconcile(seedCtx, r.SeedClientSet.Client()); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to reconcile garden access secret: %w", err)
		}

		if err := gardenerutils.MutateObjectsInSecretData(
			secretData,
			namespace.Name,
			[]string{appsv1.GroupName, batchv1.GroupName},
			// Inject generic kubeconfig
			func(obj runtime.Object) error {
				if !objectEnablesGardenKubeconfig(obj) {
					return nil
				}
				return gardenerutils.InjectGenericGardenKubeconfig(obj, genericGardenKubeconfigSecretName, gardenAccessSecret.Secret.Name, gardenerutils.VolumeMountPathGenericGardenKubeconfig)
			},
		); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to inject garden access secrets: %w", err)
		}
	} else {
		log.Info("Deleting garden access secret")
		if err := r.SeedClientSet.Client().Delete(seedCtx, gardenAccessSecret.Secret); client.IgnoreNotFound(err) != nil {
			return reconcile.Result{}, fmt.Errorf("failed to delete garden access secret: %w", err)
		}
	}

	if err := gardenerutils.MutateObjectsInSecretData(
		secretData,
		namespace.Name,
		[]string{appsv1.GroupName, batchv1.GroupName},
		// Set seed name
		func(obj runtime.Object) error {
			return kubernetesutils.VisitPodSpec(obj, func(podSpec *corev1.PodSpec) {
				kubernetesutils.VisitContainers(podSpec, func(container *corev1.Container) {
					kubernetesutils.AddEnvVar(container, corev1.EnvVar{
						Name:  v1beta1constants.EnvSeedName,
						Value: seed.Name,
					}, true)
				})
			})
		},
		r.MutateSpecForControlPlaneNodeBootstrapping,
	); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to inject garden access secrets: %w", err)
	}

	if err := managedresources.Create(
		seedCtx,
		r.SeedClientSet.Client(),
		v1beta1constants.GardenNamespace,
		controllerInstallation.Name,
		map[string]string{ctrlinstutils.LabelKeyControllerInstallationName: controllerInstallation.Name},
		false,
		v1beta1constants.SeedResourceManagerClass,
		secretData,
		nil,
		nil,
		nil,
	); err != nil {
		conditionInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionInstalled, gardencorev1beta1.ConditionFalse, "InstallationFailed", fmt.Sprintf("Creation of ManagedResource %q failed: %+v", controllerInstallation.Name, err))
		return reconcile.Result{}, err
	}

	if conditionInstalled.Status == gardencorev1beta1.ConditionUnknown {
		// initially set condition to Pending
		// care controller will update condition based on 'ResourcesApplied' condition of ManagedResource
		conditionInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionInstalled, gardencorev1beta1.ConditionFalse, "InstallationPending", fmt.Sprintf("Installation of ManagedResource %q is still pending.", controllerInstallation.Name))
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) delete(
	gardenCtx context.Context,
	seedCtx context.Context,
	log logr.Logger,
	controllerInstallation *gardencorev1beta1.ControllerInstallation,
) (
	reconcile.Result,
	error,
) {
	var (
		newConditions      = v1beta1helper.MergeConditions(controllerInstallation.Status.Conditions, v1beta1helper.InitConditionWithClock(r.Clock, gardencorev1beta1.ControllerInstallationValid), v1beta1helper.InitConditionWithClock(r.Clock, gardencorev1beta1.ControllerInstallationInstalled))
		conditionValid     = newConditions[0]
		conditionInstalled = newConditions[1]
	)

	defer func() {
		if err := patchConditions(gardenCtx, r.GardenClient, controllerInstallation, conditionValid, conditionInstalled); client.IgnoreNotFound(err) != nil {
			log.Error(err, "Failed to patch conditions")
		}
	}()

	seed := &gardencorev1beta1.Seed{}
	if err := r.GardenClient.Get(gardenCtx, client.ObjectKey{Name: controllerInstallation.Spec.SeedRef.Name}, seed); err != nil {
		if apierrors.IsNotFound(err) {
			conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionFalse, "SeedNotFound", fmt.Sprintf("Referenced Seed does not exist: %+v", err))
		} else {
			conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionUnknown, "SeedReadError", fmt.Sprintf("Referenced Seed cannot be read: %+v", err))
		}
		return reconcile.Result{}, err
	}

	mr := &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerInstallation.Name,
			Namespace: v1beta1constants.GardenNamespace,
		},
	}

	if err := client.IgnoreNotFound(managedresources.Delete(seedCtx, r.SeedClientSet.Client(), mr.Namespace, mr.Name, false)); err != nil {
		log.Info("Deletion of ManagedResource and its secrets failed", "managedResource", client.ObjectKeyFromObject(mr))
		conditionInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionInstalled, gardencorev1beta1.ConditionFalse, "DeletionFailed", fmt.Sprintf("Deletion of ManagedResource %q and its secrets failed: %+v", controllerInstallation.Name, err))
		return reconcile.Result{}, err
	}

	if err := r.SeedClientSet.Client().Get(seedCtx, client.ObjectKeyFromObject(mr), mr); err == nil {
		log.Info("Deletion of ManagedResource is still pending", "managedResource", client.ObjectKeyFromObject(mr))
		msg := fmt.Sprintf("Deletion of ManagedResource %q is still pending.", controllerInstallation.Name)
		conditionInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionInstalled, gardencorev1beta1.ConditionFalse, "DeletionPending", msg)
		return reconcile.Result{RequeueAfter: RequeueDurationWhenResourceDeletionStillPresent}, nil
	} else if !apierrors.IsNotFound(err) {
		conditionInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionInstalled, gardencorev1beta1.ConditionFalse, "DeletionFailed", fmt.Sprintf("Deletion of ManagedResource %q failed: %+v", controllerInstallation.Name, err))
		return reconcile.Result{}, err
	}

	namespace := getNamespaceForControllerInstallation(controllerInstallation)
	if err := r.SeedClientSet.Client().Delete(seedCtx, namespace); err == nil || apierrors.IsConflict(err) {
		log.Info("Deletion of Namespace is still pending", "namespace", client.ObjectKeyFromObject(namespace))

		msg := fmt.Sprintf("Deletion of Namespace %q is still pending.", namespace.Name)
		conditionInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionInstalled, gardencorev1beta1.ConditionFalse, "DeletionPending", msg)
		return reconcile.Result{RequeueAfter: RequeueDurationWhenResourceDeletionStillPresent}, nil
	} else if !apierrors.IsNotFound(err) {
		conditionInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionInstalled, gardencorev1beta1.ConditionFalse, "DeletionFailed", fmt.Sprintf("Deletion of Namespace %q failed: %+v", namespace.Name, err))
		return reconcile.Result{}, err
	}

	conditionInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionInstalled, gardencorev1beta1.ConditionFalse, "DeletionSuccessful", "Deletion of old resources succeeded.")

	gardenClusterServiceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Name:      v1beta1constants.ExtensionGardenServiceAccountPrefix + controllerInstallation.Name,
		Namespace: gardenerutils.ComputeGardenNamespace(seed.Name),
	}}
	if err := r.GardenClient.Delete(gardenCtx, gardenClusterServiceAccount); client.IgnoreNotFound(err) != nil {
		conditionInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionInstalled, gardencorev1beta1.ConditionFalse, "DeletionFailed", fmt.Sprintf("Deletion of ServiceAccount %q in garden cluster failed: %+v", client.ObjectKeyFromObject(gardenClusterServiceAccount), err))
		return reconcile.Result{}, err
	}

	if controllerutil.ContainsFinalizer(controllerInstallation, finalizerName) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(gardenCtx, r.GardenClient, controllerInstallation, finalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func patchConditions(ctx context.Context, c client.StatusClient, controllerInstallation *gardencorev1beta1.ControllerInstallation, conditions ...gardencorev1beta1.Condition) error {
	patch := client.StrategicMergeFrom(controllerInstallation.DeepCopy())
	controllerInstallation.Status.Conditions = v1beta1helper.MergeConditions(controllerInstallation.Status.Conditions, conditions...)
	return c.Status().Patch(ctx, controllerInstallation, patch)
}

func getNamespaceForControllerInstallation(controllerInstallation *gardencorev1beta1.ControllerInstallation) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: gardenerutils.NamespaceNameForControllerInstallation(controllerInstallation),
		},
	}
}

func (r *Reconciler) reconcileGenericGardenKubeconfig(ctx context.Context, namespace string) (string, error) {
	var (
		address *string
		caCert  []byte
	)

	if gcc := r.Config.GardenClientConnection; gcc != nil {
		address = gcc.GardenClusterAddress
		caCert = gcc.GardenClusterCACert
	}

	restConfig := gardenerutils.PrepareGardenClientRestConfig(r.GardenConfig, address, caCert)

	kubeconfig, err := clientcmd.Write(clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{"garden": {
			Server:                   restConfig.Host,
			InsecureSkipTLSVerify:    restConfig.Insecure,
			CertificateAuthorityData: restConfig.CAData,
		}},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{"extension": {
			TokenFile: gardenerutils.PathGardenToken,
		}},
		Contexts: map[string]*clientcmdapi.Context{"garden": {
			Cluster:  "garden",
			AuthInfo: "extension",
		}},
		CurrentContext: "garden",
	})
	if err != nil {
		return "", fmt.Errorf("failed to serialize generic garden kubeconfig: %w", err)
	}

	kubeconfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.SecretNameGenericGardenKubeconfig,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			secretsutils.DataKeyKubeconfig: kubeconfig,
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(kubeconfigSecret))

	return kubeconfigSecret.Name, client.IgnoreAlreadyExists(r.SeedClientSet.Client().Create(ctx, kubeconfigSecret))
}

func needsGardenKubeconfig(injectGardenKubeconfig bool, secretData map[string][]byte) (bool, error) {
	if !injectGardenKubeconfig {
		return false, nil
	}

	objects, err := gardenerutils.ObjectsInSecretData(secretData)
	if err != nil {
		return false, fmt.Errorf("failed reading objects in secret data: %w", err)
	}

	for _, o := range objects {
		switch o.GetObjectKind().GroupVersionKind() {
		case corev1.SchemeGroupVersion.WithKind("Pod"):
			fallthrough
		case appsv1.SchemeGroupVersion.WithKind("Deployment"):
			fallthrough
		case appsv1beta2.SchemeGroupVersion.WithKind("Deployment"):
			fallthrough
		case appsv1beta1.SchemeGroupVersion.WithKind("Deployment"):
			fallthrough
		case appsv1.SchemeGroupVersion.WithKind("StatefulSet"):
			fallthrough
		case appsv1beta2.SchemeGroupVersion.WithKind("StatefulSet"):
			fallthrough
		case appsv1beta1.SchemeGroupVersion.WithKind("StatefulSet"):
			fallthrough
		case appsv1.SchemeGroupVersion.WithKind("DaemonSet"):
			fallthrough
		case appsv1beta2.SchemeGroupVersion.WithKind("DaemonSet"):
			fallthrough
		case batchv1.SchemeGroupVersion.WithKind("Job"):
			fallthrough
		case batchv1.SchemeGroupVersion.WithKind("CronJob"):
			fallthrough
		case batchv1beta1.SchemeGroupVersion.WithKind("CronJob"):
			if objectEnablesGardenKubeconfig(o) {
				return true, nil
			}
		}
	}

	return false, nil
}

func objectEnablesGardenKubeconfig(o runtime.Object) bool {
	obj, ok := o.(client.Object)
	if !ok {
		return false
	}

	v, ok := obj.GetLabels()[v1beta1constants.LabelInjectGardenKubeconfig]
	return !ok || v == "true"
}

// MutateSpecForControlPlaneNodeBootstrapping adapts host network, replicas, tolerations and usable ports range for
// autonomous shoot clusters if necessary.
func (r *Reconciler) MutateSpecForControlPlaneNodeBootstrapping(obj runtime.Object) error {
	if !r.BootstrapControlPlaneNode {
		return nil
	}

	if deployment, ok := obj.(*appsv1.Deployment); ok {
		deployment.Spec.Replicas = ptr.To(int32(1))
	}
	return kubernetesutils.VisitPodSpec(obj, func(podSpec *corev1.PodSpec) {
		podSpec.HostNetwork = r.BootstrapControlPlaneNode
		podSpec.Tolerations = append(podSpec.Tolerations,
			corev1.Toleration{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
			corev1.Toleration{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute},
		)
	})
}

// CalculateUsablePorts returns the next usable port range for the next controller installation.
func (r *Reconciler) CalculateUsablePorts() ([]int, error) {
	var ports []int
	for i := 0; i < usablePortsRangeSize; i++ {
		p, _, err := netutils.SuggestPort("")
		if err != nil {
			return nil, fmt.Errorf("failed to find a usable port: %w", err)
		}
		ports = append(ports, p)
	}
	return ports, nil
}
