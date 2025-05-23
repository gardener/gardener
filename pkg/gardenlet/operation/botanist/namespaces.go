// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	podsecurityadmissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/shoot/namespaces"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// DeployControlPlaneNamespace creates a namespace in the Seed cluster which is used to deploy all the control plane
// components for the Shoot cluster. Moreover, the cloud provider configuration and all the secrets will be
// stored as ConfigMaps/Secrets.
func (b *Botanist) DeployControlPlaneNamespace(ctx context.Context) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: b.Shoot.ControlPlaneNamespace,
		},
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, b.SeedClientSet.Client(), namespace, func() error {
		requiredExtensions, err := b.getShootRequiredExtensionTypes(ctx)
		if err != nil {
			return err
		}

		metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, v1beta1constants.ShootUID, string(b.Shoot.GetInfo().Status.UID))
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.GardenRole, v1beta1constants.GardenRoleShoot)
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.LabelSeedProvider, b.Seed.GetInfo().Spec.Provider.Type)
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.LabelShootProvider, b.Shoot.GetInfo().Spec.Provider.Type)
		if b.Shoot.GetInfo().Spec.Networking != nil && b.Shoot.GetInfo().Spec.Networking.Type != nil {
			metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.LabelNetworkingProvider, *b.Shoot.GetInfo().Spec.Networking.Type)
		}

		// Remove all old extension labels before reconciling the new extension labels.
		for k := range namespace.Labels {
			if strings.HasPrefix(k, v1beta1constants.LabelExtensionPrefix) {
				delete(namespace.Labels, k)
			}
		}
		for extensionType := range requiredExtensions {
			metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.LabelExtensionPrefix+extensionType, "true")
		}

		metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.LabelBackupProvider, b.Seed.GetInfo().Spec.Provider.Type)
		if b.Seed.GetInfo().Spec.Backup != nil {
			metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.LabelBackupProvider, b.Seed.GetInfo().Spec.Backup.Provider)
		}

		metav1.SetMetaDataLabel(&namespace.ObjectMeta, podsecurityadmissionapi.EnforceLevelLabel, string(podsecurityadmissionapi.LevelPrivileged))
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")

		existingFailureToleranceType, failureToleranceTypeExisting := namespace.Annotations[resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType]

		shootFailureToleranceType := v1beta1helper.GetFailureToleranceType(b.Shoot.GetInfo())
		if shootFailureToleranceType == nil {
			metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType, "")
		} else {
			metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType, string(*shootFailureToleranceType))
		}

		newFailureToleranceType := namespace.Annotations[resourcesv1alpha1.HighAvailabilityConfigFailureToleranceType]

		if seedZones := b.Seed.GetInfo().Spec.Provider.Zones; len(seedZones) > 0 &&
			(!failureToleranceTypeExisting || existingFailureToleranceType != newFailureToleranceType) {
			zones, err := calculateShootZones(ctx, b.SeedClientSet.Client(), b.Logger, namespace, shootFailureToleranceType, seedZones)
			if err != nil {
				return err
			}

			metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, strings.Join(zones, ","))
		}

		return nil
	}); err != nil {
		return err
	}

	b.SeedNamespaceObject = namespace
	return nil
}

// calculateShootZones calculates and picks zones for the shoot cluster based on the passed 'failureToleranceType'.
// 'failureToleranceType' == 'zone' -> 3 zones
// 'failureToleranceType' != 'zone' -> 1 zone
// If there are volumes in already used zones, then these zones will be part of the list too.
func calculateShootZones(ctx context.Context, cl client.Client, log logr.Logger, namespace *corev1.Namespace, failureToleranceType *gardencorev1beta1.FailureToleranceType, seedZones []string) ([]string, error) {
	zonesToSelect := 1
	if failureToleranceType != nil && *failureToleranceType == gardencorev1beta1.FailureToleranceTypeZone {
		zonesToSelect = 3
	}

	chosenZones := sets.New[string]()

	if zones, ok := namespace.Annotations[resourcesv1alpha1.HighAvailabilityConfigZones]; ok {
		chosenZones.Insert(strings.Split(zones, ",")...)
	}

	// The zones annotation is used to add a node affinity to pods and pin them to exactly those zones part of
	// the annotation's value. However, existing clusters might already run in multiple zones. In particular,
	// if they have created their volumes in multiple zones already, we cannot change this unless we delete and
	// recreate the disks. This is nothing we want to do automatically, so let's find the existing volumes and
	// use their zones from now on.
	// As a consequence, even shoots w/o failure tolerance type 'zone' might be pinned to multiple zones.
	pvcList := &corev1.PersistentVolumeClaimList{}
	if err := cl.List(ctx, pvcList, client.InNamespace(namespace.Name)); err != nil {
		return nil, fmt.Errorf("failed listing PVCs: %w", err)
	}

	for _, pvc := range pvcList.Items {
		// Skip handling if PV has not been created yet.
		if pvc.Spec.VolumeName == "" {
			continue
		}

		pv := &corev1.PersistentVolume{}
		if err := cl.Get(ctx, client.ObjectKey{Name: pvc.Spec.VolumeName}, pv); err != nil {
			return nil, fmt.Errorf("failed getting PV %s: %w", pvc.Spec.VolumeName, err)
		}

		pvNodeAffinity := pv.Spec.NodeAffinity
		if pvNodeAffinity == nil || pvNodeAffinity.Required == nil {
			continue
		}

		for _, term := range pvNodeAffinity.Required.NodeSelectorTerms {
			zonesFromTerm := ExtractZonesFromNodeSelectorTerm(term)
			if len(zonesFromTerm) == 0 {
				continue
			}

			seedZoneSet := sets.New(seedZones...)
			for _, zoneFromTerm := range zonesFromTerm {
				// Only add zone if it is found in the seed spec.
				// Zone name mismatches are handled by provider extensions, e.g. see https://github.com/gardener/gardener-extension-provider-azure/pull/602.
				if !seedZoneSet.Has(zoneFromTerm) {
					continue
				}
				log.Info("Found existing zone due to volume", "zone", zoneFromTerm, "persistentVolume", client.ObjectKeyFromObject(pv))
				chosenZones.Insert(zoneFromTerm)
			}
		}
	}

	if len(seedZones) < zonesToSelect-chosenZones.Len() {
		return nil, fmt.Errorf("cannot select %d zones for shoot because seed only specifies %d zones in its specification", zonesToSelect, len(seedZones))
	}

	for chosenZones.Len() < zonesToSelect {
		chosenZones.Insert(seedZones[rand.Intn(len(seedZones))])
	}

	return sets.List(chosenZones), nil
}

// ExtractZonesFromNodeSelectorTerm extracts the zones from given term.
func ExtractZonesFromNodeSelectorTerm(term corev1.NodeSelectorTerm) []string {
	zones := sets.New[string]()
	for _, matchExpression := range term.MatchExpressions {
		if matchExpression.Operator != corev1.NodeSelectorOpIn {
			continue
		}

		key := matchExpression.Key
		// Only consider labels with 'topology.{provider-specific-string}/zone' or "failure-domain.beta.kubernetes.io/zone" which should match most of the cases.
		if (strings.HasPrefix(key, "topology.") && strings.HasSuffix(key, "/zone")) ||
			key == corev1.LabelFailureDomainBetaZone {
			zones.Insert(matchExpression.Values...)
		}
	}
	return zones.UnsortedList()
}

// DeleteSeedNamespace deletes the namespace in the Seed cluster which holds the control plane components. The built-in
// garbage collection in Kubernetes will automatically delete all resources which belong to this namespace. This
// comprises volumes and load balancers as well.
func (b *Botanist) DeleteSeedNamespace(ctx context.Context) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: b.Shoot.ControlPlaneNamespace,
		},
	}

	err := b.SeedClientSet.Client().Delete(ctx, namespace, kubernetes.DefaultDeleteOptions...)
	if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
		return nil
	}

	return err
}

// WaitUntilSeedNamespaceDeleted waits until the namespace of the Shoot cluster within the Seed cluster is deleted.
func (b *Botanist) WaitUntilSeedNamespaceDeleted(ctx context.Context) error {
	return retry.UntilTimeout(ctx, 5*time.Second, 900*time.Second, func(ctx context.Context) (done bool, err error) {
		if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKey{Name: b.Shoot.ControlPlaneNamespace}, &corev1.Namespace{}); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}
		b.Logger.Info("Waiting until the namespace has been cleaned up and deleted in the Seed cluster", "namespaceName", b.Shoot.ControlPlaneNamespace)
		return retry.MinorError(fmt.Errorf("namespace %q is not yet cleaned up", b.Shoot.ControlPlaneNamespace))
	})
}

// DefaultShootNamespaces returns a deployer for the shoot namespaces.
func (b *Botanist) DefaultShootNamespaces() component.DeployWaiter {
	return namespaces.New(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, b.Shoot.GetInfo().Spec.Provider.Workers)
}

// getShootRequiredExtensionTypes returns all extension types that are enabled or explicitly disabled for the shoot.
// The function considers only extensions of kind `Extension`.
func (b *Botanist) getShootRequiredExtensionTypes(ctx context.Context) (sets.Set[string], error) {
	controllerRegistrationList := &gardencorev1beta1.ControllerRegistrationList{}
	if err := b.GardenClient.List(ctx, controllerRegistrationList); err != nil {
		return nil, err
	}

	types := sets.Set[string]{}
	for _, reg := range controllerRegistrationList.Items {
		for _, res := range reg.Spec.Resources {
			if res.Kind == extensionsv1alpha1.ExtensionResource && ptr.Deref(res.GloballyEnabled, false) {
				types.Insert(res.Type)
			}
		}
	}

	for _, extension := range b.Shoot.GetInfo().Spec.Extensions {
		if ptr.Deref(extension.Disabled, false) {
			types.Delete(extension.Type)
		} else {
			types.Insert(extension.Type)
		}
	}

	return types, nil
}
