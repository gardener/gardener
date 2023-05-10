// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerinstallation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	ctrlinstutils "github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/utils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const finalizerName = "core.gardener.cloud/controllerinstallation"

// RequeueDurationWhenResourceDeletionStillPresent is the duration used for requeueing when owned resources are still in
// the process of being deleted when deleting a ControllerInstallation.
var RequeueDurationWhenResourceDeletionStillPresent = 30 * time.Second

// Reconciler reconciles ControllerInstallations and deploys them into the seed cluster.
type Reconciler struct {
	GardenClient          client.Client
	SeedClientSet         kubernetes.Interface
	Config                config.ControllerInstallationControllerConfiguration
	Clock                 clock.Clock
	Identity              *gardencorev1beta1.Gardener
	GardenClusterIdentity string
}

// Reconcile reconciles ControllerInstallations and deploys them into the seed cluster.
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

	var providerConfig *runtime.RawExtension
	if deploymentRef := controllerInstallation.Spec.DeploymentRef; deploymentRef != nil {
		controllerDeployment := &gardencorev1beta1.ControllerDeployment{}
		if err := r.GardenClient.Get(gardenCtx, kubernetesutils.Key(deploymentRef.Name), controllerDeployment); err != nil {
			return reconcile.Result{}, err
		}
		providerConfig = &controllerDeployment.ProviderConfig
	}

	var helmDeployment struct {
		// chart is a Helm chart tarball.
		Chart []byte `json:"chart,omitempty"`
		// Values is a map of values for the given chart.
		Values map[string]interface{} `json:"values,omitempty"`
	}

	if err := json.Unmarshal(providerConfig.Raw, &helmDeployment); err != nil {
		conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionFalse, "ChartInformationInvalid", fmt.Sprintf("chart Information cannot be unmarshalled: %+v", err))
		return reconcile.Result{}, err
	}

	namespace := getNamespaceForControllerInstallation(controllerInstallation)
	if _, err := controllerutils.GetAndCreateOrMergePatch(seedCtx, r.SeedClientSet.Client(), namespace, func() error {
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.GardenRole, v1beta1constants.GardenRoleExtension)
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.LabelControllerRegistrationName, controllerRegistration.Name)
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")
		metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(seed.Spec.Provider.Zones, ","))
		return nil
	}); err != nil {
		return reconcile.Result{}, err
	}

	// TODO(timuthy, rfranzke): Drop this code when the FullNetworkPoliciesInRuntimeCluster feature gate gets promoted to GA.
	if err := r.reconcileNetworkPoliciesInSeed(seedCtx, namespace.Name); err != nil {
		return reconcile.Result{}, err
	}

	var (
		volumeProvider  string
		volumeProviders []gardencorev1beta1.SeedVolumeProvider
	)

	if seed.Spec.Volume != nil {
		volumeProviders = seed.Spec.Volume.Providers
		if len(seed.Spec.Volume.Providers) > 0 {
			volumeProvider = seed.Spec.Volume.Providers[0].Name
		}
	}

	if seed.Status.ClusterIdentity == nil {
		return reconcile.Result{}, fmt.Errorf("cluster-identity of seed '%s' not set", seed.Name)
	}
	seedClusterIdentity := *seed.Status.ClusterIdentity

	// Mix-in some standard values for garden and seed.
	gardenerValues := map[string]interface{}{
		"gardener": map[string]interface{}{
			"version": r.Identity.Version,
			"garden": map[string]interface{}{
				"clusterIdentity": r.GardenClusterIdentity,
			},
			"seed": map[string]interface{}{
				"name":            seed.Name,
				"clusterIdentity": seedClusterIdentity,
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
		},
	}

	release, err := r.SeedClientSet.ChartRenderer().RenderArchive(helmDeployment.Chart, controllerRegistration.Name, namespace.Name, utils.MergeMaps(helmDeployment.Values, gardenerValues))
	if err != nil {
		conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionFalse, "ChartCannotBeRendered", fmt.Sprintf("chart rendering process failed: %+v", err))
		return reconcile.Result{}, err
	}
	conditionValid = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionValid, gardencorev1beta1.ConditionTrue, "RegistrationValid", "chart could be rendered successfully.")

	if err := managedresources.Create(
		seedCtx,
		r.SeedClientSet.Client(),
		v1beta1constants.GardenNamespace,
		controllerInstallation.Name,
		map[string]string{ctrlinstutils.LabelKeyControllerInstallationName: controllerInstallation.Name},
		false,
		v1beta1constants.SeedResourceManagerClass,
		release.AsSecretData(),
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
	if err := r.SeedClientSet.Client().Delete(seedCtx, mr); err == nil {
		log.Info("Deletion of ManagedResource is still pending", "managedResource", client.ObjectKeyFromObject(mr))

		msg := fmt.Sprintf("Deletion of ManagedResource %q is still pending.", controllerInstallation.Name)
		conditionInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionInstalled, gardencorev1beta1.ConditionFalse, "DeletionPending", msg)
		return reconcile.Result{RequeueAfter: RequeueDurationWhenResourceDeletionStillPresent}, nil
	} else if !apierrors.IsNotFound(err) {
		conditionInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionInstalled, gardencorev1beta1.ConditionFalse, "DeletionFailed", fmt.Sprintf("Deletion of ManagedResource %q failed: %+v", controllerInstallation.Name, err))
		return reconcile.Result{}, err
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      controllerInstallation.Name,
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
	if err := r.SeedClientSet.Client().Delete(seedCtx, secret); client.IgnoreNotFound(err) != nil {
		conditionInstalled = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionInstalled, gardencorev1beta1.ConditionFalse, "DeletionFailed", fmt.Sprintf("Deletion of ManagedResource secret %q failed: %+v", controllerInstallation.Name, err))
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

func (r *Reconciler) reconcileNetworkPoliciesInSeed(ctx context.Context, namespace string) error {
	var (
		peers = []networkingv1.NetworkPolicyPeer{
			{PodSelector: &metav1.LabelSelector{}, NamespaceSelector: &metav1.LabelSelector{}},
			{IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"}},
			{IPBlock: &networkingv1.IPBlock{CIDR: "::/0"}},
		}

		allowAllTrafficNetworkPolicy = &networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "gardenlet-allow-all-traffic",
				Namespace: namespace,
			},
			Spec: networkingv1.NetworkPolicySpec{
				Ingress:     []networkingv1.NetworkPolicyIngressRule{{From: peers}},
				Egress:      []networkingv1.NetworkPolicyEgressRule{{To: peers}},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress},
			},
		}
	)

	if !features.DefaultFeatureGate.Enabled(features.FullNetworkPoliciesInRuntimeCluster) {
		return client.IgnoreAlreadyExists(r.SeedClientSet.Client().Create(ctx, allowAllTrafficNetworkPolicy))
	}
	return client.IgnoreNotFound(r.SeedClientSet.Client().Delete(ctx, allowAllTrafficNetworkPolicy))
}
