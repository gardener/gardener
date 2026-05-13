// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerinstallation

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	// SeedSpecHash is a constant for a label on `ControllerInstallation`s (similar to `pod-template-hash` on `Pod`s).
	SeedSpecHash = "seed-spec-hash"
	// ShootSpecHash is a constant for a label on `ControllerInstallation`s (similar to `pod-template-hash` on `Pod`s).
	ShootSpecHash = "shoot-spec-hash"
	// ControllerDeploymentHash is a constant for a label on `ControllerInstallation`s (similar to `pod-template-hash`
	// on `Pod`s).
	ControllerDeploymentHash = "deployment-hash"
	// RegistrationSpecHash is a constant for a label on `ControllerInstallation`s (similar to `pod-template-hash` on
	// Pod`s).
	RegistrationSpecHash = "registration-spec-hash"
	// SeedRefName is a constant for a label on `ControllerInstallation`s created by the seed reconciler for
	// self-hosted-shoot seeds. Its value is the seed name. It allows the shoot reconciler to distinguish these
	// ControllerInstallations (which use .spec.shootRef) from those it owns.
	SeedRefName = "seed-ref-name"
)

// Kind is a string alias.
type Kind string

const (
	// SeedKind indicates that the controller acts on Seeds.
	SeedKind Kind = "seed"
	// ShootKind indicates that the controller acts on self-hosted Shoots.
	ShootKind Kind = "shoot"
)

// Reconciler determines which ControllerRegistrations are required for a given Seed or Shoot by checking all objects in
// the garden cluster, that need to be considered for that Seed or Shoot (e.g. because they reference it). It then
// deploys wanted and deletes unneeded ControllerInstallations accordingly. Seeds or Shoots get enqueued by updates to
// relevant (referencing) objects, e.g. Shoots, BackupBuckets, etc.
type Reconciler struct {
	APIReader           client.Reader
	Client              client.Client
	NewTargetObjectFunc func() client.Object
	Kind                Kind
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	obj := r.NewTargetObjectFunc()
	if err := r.Client.Get(ctx, request.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	log.Info("Reconciling")

	controllerRegistrationList := &gardencorev1beta1.ControllerRegistrationList{}
	if err := r.Client.List(ctx, controllerRegistrationList); err != nil {
		return reconcile.Result{}, err
	}

	// Live lookup to prevent working on a stale cache and trying to create multiple installations for the same
	// registration/seed/shoot combination.
	controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
	if err := r.APIReader.List(ctx, controllerInstallationList); err != nil {
		return reconcile.Result{}, err
	}

	backupBucketList := &gardencorev1beta1.BackupBucketList{}
	if err := r.Client.List(ctx, backupBucketList, backupBucketFieldSelector(obj, r.Kind)...); err != nil {
		return reconcile.Result{}, err
	}
	backupBucketNameToObject := make(map[string]*gardencorev1beta1.BackupBucket, len(backupBucketList.Items))
	for _, backupBucket := range backupBucketList.Items {
		backupBucketNameToObject[backupBucket.Name] = &backupBucket
	}

	backupEntryList := &gardencorev1beta1.BackupEntryList{}
	if err := r.Client.List(ctx, backupEntryList, backupEntryFieldSelector(obj, r.Kind)...); err != nil {
		return reconcile.Result{}, err
	}

	shootList, err := getShoots(ctx, r.Client, obj, r.Kind)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed listing shoots: %w", err)
	}

	var (
		controllerRegistrations    = computeControllerRegistrationMaps(controllerRegistrationList)
		wantedKindTypeCombinations = sets.
						New[string]().
						Union(computeKindTypesForBackupBuckets(backupBucketNameToObject, obj, r.Kind)).
						Union(computeKindTypesForBackupEntries(log, backupEntryList, backupBucketNameToObject))
	)

	wantedKindTypeCombinationForHostedShoots, err := computeKindTypesForHostedShoots(ctx, log, r.Client, obj, r.Kind, controllerRegistrationList, shootList)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed computing kind/types for shoots: %w", err)
	}
	wantedKindTypeCombinations = wantedKindTypeCombinations.Union(wantedKindTypeCombinationForHostedShoots)

	// selfHostedShoot is non-nil when the seed being reconciled is a self-hosted shoot cluster. In that case the
	// shoot reconciler already manages ControllerInstallations with .spec.shootRef for this shoot. The seed reconciler
	// subtracts kind/types exclusively needed by the self-hosted shoot (i.e., those not also needed by shoots hosted
	// on this seed and not independently needed by the seed itself) to avoid creating duplicate ControllerInstallations
	// for them. Kind/types needed by both the self-hosted shoot and hosted shoots or the seed itself remain so that
	// the seed reconciler can set .spec.seedRef on the existing shoot-owned ControllerInstallations.
	var (
		selfHostedShoot                  *gardencorev1beta1.Shoot
		wantedKindTypeCombinationForSeed sets.Set[string]
	)

	if r.Kind == SeedKind {
		seed, ok := obj.(*gardencorev1beta1.Seed)
		if !ok {
			return reconcile.Result{}, fmt.Errorf("cannot convert object of type %T to *gardencorev1beta1.Seed", obj)
		}
		wantedKindTypeCombinationForSeed = computeKindTypesForSeed(seed, controllerRegistrationList)
		wantedKindTypeCombinations = wantedKindTypeCombinations.Union(wantedKindTypeCombinationForSeed)

		if metav1.HasLabel(seed.ObjectMeta, v1beta1constants.LabelSelfHostedShootCluster) {
			shoot := &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: seed.Name, Namespace: v1beta1constants.GardenNamespace}}
			if err := r.Client.Get(ctx, client.ObjectKeyFromObject(shoot), shoot); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed getting Shoot for self-hosted seed: %w", err)
			}
			selfHostedShoot = shoot
		}
	}

	var shootNeededRegistrationNames sets.Set[string]
	if selfHostedShoot != nil {
		// Subtract all kind/type combinations already managed by the shoot reconciler for this shoot so that the
		// seed reconciler only creates ControllerInstallations for extensions exclusively needed by the seed role.
		// This covers the shoot's own spec, plus BackupBuckets and BackupEntries referencing the shoot.
		shootBackupBucketList := &gardencorev1beta1.BackupBucketList{}
		if err := r.Client.List(ctx, shootBackupBucketList, backupBucketFieldSelector(selfHostedShoot, ShootKind)...); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed listing BackupBuckets for self-hosted shoot: %w", err)
		}
		shootBackupBucketNameToObject := make(map[string]*gardencorev1beta1.BackupBucket, len(shootBackupBucketList.Items))
		for _, backupBucket := range shootBackupBucketList.Items {
			shootBackupBucketNameToObject[backupBucket.Name] = &backupBucket
		}

		shootBackupEntryList := &gardencorev1beta1.BackupEntryList{}
		if err := r.Client.List(ctx, shootBackupEntryList, backupEntryFieldSelector(selfHostedShoot, ShootKind)...); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed listing BackupEntries for self-hosted shoot: %w", err)
		}

		wantedKindTypeCombinationForSelfHostedShoot := sets.New[string]().
			Union(computeKindTypesForBackupBuckets(shootBackupBucketNameToObject, selfHostedShoot, ShootKind)).
			Union(computeKindTypesForBackupEntries(log, shootBackupEntryList, shootBackupBucketNameToObject)).
			Union(gardenerutils.ComputeRequiredExtensionsForShoot(selfHostedShoot, nil, controllerRegistrationList, nil, nil))

		// Subtract kind/types that are exclusively needed by the self-hosted shoot (not shared with hosted
		// shoots or the seed) since they must not be installed on the hosting seed.
		kindTypesExclusivelyNeededBySelfHostedShoot := wantedKindTypeCombinationForSelfHostedShoot.
			Difference(wantedKindTypeCombinationForHostedShoots).
			Difference(wantedKindTypeCombinationForSeed)
		wantedKindTypeCombinations = wantedKindTypeCombinations.Difference(kindTypesExclusivelyNeededBySelfHostedShoot)

		shootNeededRegistrationNames = registrationNamesForKindTypes(wantedKindTypeCombinationForSelfHostedShoot, controllerRegistrations)
	}

	wantedControllerRegistrationNames, err := computeWantedControllerRegistrationNames(wantedKindTypeCombinations, controllerInstallationList, controllerRegistrations, len(shootList), obj, r.Kind, selfHostedShoot)
	if err != nil {
		return reconcile.Result{}, err
	}

	registrationNameToInstallation, err := computeRegistrationNameToInstallationMap(controllerInstallationList, controllerRegistrations, obj, r.Kind, selfHostedShoot)
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := deployNeededInstallations(ctx, log, r.Client, obj, r.Kind, selfHostedShoot, wantedControllerRegistrationNames, shootNeededRegistrationNames, controllerRegistrations, registrationNameToInstallation); err != nil {
		return reconcile.Result{}, err
	}

	if err := deleteUnneededInstallations(ctx, log, r.Client, r.Kind, selfHostedShoot, wantedControllerRegistrationNames, shootNeededRegistrationNames, registrationNameToInstallation); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// computeKindTypesForBackupBucket computes the list of wanted kind/type combinations for extension resources based on
// the list of existing BackupBucket resources.
func computeKindTypesForBackupBuckets(
	backupBucketNameToObject map[string]*gardencorev1beta1.BackupBucket,
	obj client.Object,
	kind Kind,
) sets.Set[string] {
	wantedKindTypeCombinations := sets.New[string]()

	for _, backupBucket := range backupBucketNameToObject {
		if (kind == SeedKind && ptr.Deref(backupBucket.Spec.SeedName, "") == obj.GetName()) ||
			(kind == ShootKind && backupBucket.Spec.ShootRef != nil && backupBucket.Spec.ShootRef.Name == obj.GetName() && backupBucket.Spec.ShootRef.Namespace == obj.GetNamespace()) {
			wantedKindTypeCombinations.Insert(gardenerutils.ExtensionsID(extensionsv1alpha1.BackupBucketResource, backupBucket.Spec.Provider.Type))
		}
	}

	return wantedKindTypeCombinations
}

// computeKindTypesForBackupEntries computes the list of wanted kind/type combinations for extension resources based on
// the list of existing BackupEntry resources.
func computeKindTypesForBackupEntries(
	log logr.Logger,
	backupEntryList *gardencorev1beta1.BackupEntryList,
	backupBucketNameToObject map[string]*gardencorev1beta1.BackupBucket,
) sets.Set[string] {
	wantedKindTypeCombinations := sets.New[string]()

	for _, backupEntry := range backupEntryList.Items {
		bucket, ok := backupBucketNameToObject[backupEntry.Spec.BucketName]
		if !ok {
			log.Error(fmt.Errorf("BackupBucket not found in list"), "Couldn't find referenced BackupBucket for BackupEntry", "backupBucketName", backupEntry.Spec.BucketName, "backupEntry", client.ObjectKeyFromObject(&backupEntry))
			continue
		}

		wantedKindTypeCombinations.Insert(gardenerutils.ExtensionsID(extensionsv1alpha1.BackupEntryResource, bucket.Spec.Provider.Type))
	}

	return wantedKindTypeCombinations
}

// computeKindTypesForHostedShoots computes the list of wanted kind/type combinations for extension resources based on the
// list of existing Shoot resources.
func computeKindTypesForHostedShoots(
	ctx context.Context,
	log logr.Logger,
	reader client.Reader,
	obj client.Object,
	kind Kind,
	controllerRegistrationList *gardencorev1beta1.ControllerRegistrationList,
	shootList []gardencorev1beta1.Shoot,
) (sets.Set[string], error) {
	if kind == ShootKind {
		shoot, ok := obj.(*gardencorev1beta1.Shoot)
		if !ok {
			return nil, fmt.Errorf("cannot convert object of type %T to *gardencorev1beta1.Shoot", obj)
		}

		// enable clean up of controller installations in case of shoot deletion
		if shoot.DeletionTimestamp != nil {
			return sets.New[string](), nil
		}

		return gardenerutils.ComputeRequiredExtensionsForShoot(shoot, nil, controllerRegistrationList, nil, nil), nil
	}

	var (
		wantedKindTypeCombinations = sets.New[string]()

		wg  sync.WaitGroup
		out = make(chan sets.Set[string])
	)

	seed, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return nil, fmt.Errorf("cannot convert object of type %T to *gardencorev1beta1.Seed", obj)
	}

	internalDomain, err := gardenerutils.ReadGardenInternalDomain(
		ctx,
		reader,
		gardenerutils.ComputeGardenNamespace(seed.Name),
		false,
		seed.Spec.DNS.Internal,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read internal domain from garden cluster: %w", err)
	}

	defaultDomains, err := gardenerutils.ReadGardenDefaultDomains(
		ctx,
		reader,
		gardenerutils.ComputeGardenNamespace(seed.Name),
		seed.Spec.DNS.Defaults,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to read default domains from garden cluster: %w", err)
	}

	for _, shoot := range shootList {
		wg.Add(1)
		go func(shoot *gardencorev1beta1.Shoot) {
			defer wg.Done()

			externalDomain, err := gardenerutils.ConstructExternalDomain(ctx, reader, shoot, &corev1.Secret{}, defaultDomains)
			if err != nil && !gardenerutils.IsIncompleteDNSConfigError(err) || shoot.DeletionTimestamp == nil || len(shoot.Status.UID) != 0 {
				log.Info("Could not determine external domain for shoot", "err", err, "shoot", client.ObjectKeyFromObject(shoot))
			}

			out <- gardenerutils.ComputeRequiredExtensionsForShoot(shoot, seed, controllerRegistrationList, internalDomain, externalDomain)
		}(shoot.DeepCopy())
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	for result := range out {
		wantedKindTypeCombinations = wantedKindTypeCombinations.Union(result)
	}

	return wantedKindTypeCombinations, nil
}

// computeKindTypesForSeed computes the list of wanted kind/type combinations for extension resources based on the
// Seed configuration
func computeKindTypesForSeed(
	seed *gardencorev1beta1.Seed,
	controllerRegistrationList *gardencorev1beta1.ControllerRegistrationList,
) sets.Set[string] {
	// enable clean up of controller installations in case of seed deletion
	if seed.DeletionTimestamp != nil {
		return sets.New[string]()
	}

	return gardenerutils.ComputeRequiredExtensionsForSeed(seed, controllerRegistrationList)
}

type controllerRegistration struct {
	obj                        *gardencorev1beta1.ControllerRegistration
	deployAlways               bool
	deployAlwaysExceptNoShoots bool
}

// computeControllerRegistrationMaps computes a map which maps the name of a ControllerRegistration to the
// *gardencorev1beta1.ControllerRegistration object. It also specifies whether the ControllerRegistration shall be
// always deployed.
func computeControllerRegistrationMaps(
	controllerRegistrationList *gardencorev1beta1.ControllerRegistrationList,
) map[string]controllerRegistration {
	var out = make(map[string]controllerRegistration)
	for _, cr := range controllerRegistrationList.Items {
		out[cr.Name] = controllerRegistration{
			obj:                        cr.DeepCopy(),
			deployAlways:               cr.Spec.Deployment != nil && cr.Spec.Deployment.Policy != nil && *cr.Spec.Deployment.Policy == gardencorev1beta1.ControllerDeploymentPolicyAlways,
			deployAlwaysExceptNoShoots: cr.Spec.Deployment != nil && cr.Spec.Deployment.Policy != nil && *cr.Spec.Deployment.Policy == gardencorev1beta1.ControllerDeploymentPolicyAlwaysExceptNoShoots,
		}
	}
	return out
}

// computeWantedControllerRegistrationNames computes the list of names of ControllerRegistration objects that are
// desired to be installed. The computation is performed based on a list of required kind/type combinations and the
// proper mapping to existing ControllerRegistration objects. Additionally, all names in the
// alwaysPolicyControllerRegistrationNames list will be returned and all currently installed and required installations.
func computeWantedControllerRegistrationNames(
	wantedKindTypeCombinations sets.Set[string],
	controllerInstallationList *gardencorev1beta1.ControllerInstallationList,
	controllerRegistrations map[string]controllerRegistration,
	numberOfShoots int,
	obj client.Object,
	kind Kind,
	selfHostedShoot *gardencorev1beta1.Shoot,
) (
	sets.Set[string],
	error,
) {
	var (
		kindTypeToControllerRegistrationNames = make(map[string][]string)
		wantedControllerRegistrationNames     = sets.New[string]()
	)

	for name, controllerRegistration := range controllerRegistrations {
		// When the seed is a self-hosted shoot, the shoot reconciler handles Always/AlwaysExceptNoShoots
		// registrations — skip them here to avoid duplicate ControllerInstallations.
		if controllerRegistration.obj.DeletionTimestamp == nil && selfHostedShoot == nil {
			if controllerRegistration.deployAlways && obj.GetDeletionTimestamp() == nil {
				wantedControllerRegistrationNames.Insert(name)
			}

			if controllerRegistration.deployAlwaysExceptNoShoots && numberOfShoots > 0 {
				wantedControllerRegistrationNames.Insert(name)
			}
		}

		for _, resource := range controllerRegistration.obj.Spec.Resources {
			id := gardenerutils.ExtensionsID(resource.Kind, resource.Type)
			kindTypeToControllerRegistrationNames[id] = append(kindTypeToControllerRegistrationNames[id], name)
		}
	}

	for _, wantedExtension := range wantedKindTypeCombinations.UnsortedList() {
		names, ok := kindTypeToControllerRegistrationNames[wantedExtension]
		if !ok {
			return nil, fmt.Errorf("need to install an extension controller for %q but no appropriate ControllerRegistration found", wantedExtension)
		}
		wantedControllerRegistrationNames.Insert(names...)
	}

	// When the seed is a self-hosted shoot, the shoot reconciler owns all ControllerInstallations for this shoot —
	// the seed reconciler must not keep any of them alive via the "installed and required" mechanism.
	if selfHostedShoot == nil {
		wantedControllerRegistrationNames.Insert(sets.List(installedAndRequiredRegistrationNames(controllerInstallationList, obj, kind))...)
	}

	if kind == ShootKind {
		return wantedControllerRegistrationNames, nil
	}
	// for seeds, filter controller registrations with non-matching seed selector
	return controllerRegistrationNamesWithMatchingSeedLabelSelector(wantedControllerRegistrationNames.UnsortedList(), controllerRegistrations, obj.GetLabels())
}

func installedAndRequiredRegistrationNames(controllerInstallationList *gardencorev1beta1.ControllerInstallationList, obj client.Object, kind Kind) sets.Set[string] {
	requiredControllerRegistrationNames := sets.New[string]()
	for _, controllerInstallation := range controllerInstallationList.Items {
		if !controllerInstallationReferencesObject(controllerInstallation, obj, kind, nil) {
			continue
		}
		if !v1beta1helper.IsControllerInstallationRequired(controllerInstallation) {
			continue
		}
		requiredControllerRegistrationNames.Insert(controllerInstallation.Spec.RegistrationRef.Name)
	}
	return requiredControllerRegistrationNames
}

// computeRegistrationNameToInstallationMap computes a map that maps the name of a ControllerRegistration to an
// existing ControllerInstallation object that references this registration.
func computeRegistrationNameToInstallationMap(
	controllerInstallationList *gardencorev1beta1.ControllerInstallationList,
	controllerRegistrations map[string]controllerRegistration,
	obj client.Object,
	kind Kind,
	selfHostedShoot *gardencorev1beta1.Shoot,
) (
	map[string]*gardencorev1beta1.ControllerInstallation,
	error,
) {
	registrationNameToInstallationName := make(map[string]*gardencorev1beta1.ControllerInstallation)

	for _, controllerInstallation := range controllerInstallationList.Items {
		if !controllerInstallationReferencesObject(controllerInstallation, obj, kind, selfHostedShoot) {
			continue
		}

		if _, ok := controllerRegistrations[controllerInstallation.Spec.RegistrationRef.Name]; !ok {
			return nil, fmt.Errorf("ControllerRegistration %q does not exist", controllerInstallation.Spec.RegistrationRef.Name)
		}

		controllerInstallationObj := controllerInstallation
		registrationNameToInstallationName[controllerInstallation.Spec.RegistrationRef.Name] = &controllerInstallationObj
	}

	return registrationNameToInstallationName, nil
}

// deployNeededInstallations takes the list of required names of ControllerRegistrations, a mapping of ControllerRegistration
// names to their actual objects, and another mapping of ControllerRegistration names to existing ControllerInstallations. It
// creates or updates ControllerInstallation objects that reference the given seed or shoot and the various desired
// ControllerRegistrations.
func deployNeededInstallations(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	obj client.Object,
	kind Kind,
	selfHostedShoot *gardencorev1beta1.Shoot,
	wantedControllerRegistrations sets.Set[string],
	shootNeededRegistrationNames sets.Set[string],
	controllerRegistrations map[string]controllerRegistration,
	registrationNameToInstallation map[string]*gardencorev1beta1.ControllerInstallation,
) error {
	for _, registrationName := range wantedControllerRegistrations.UnsortedList() {
		registrationLog := log.WithValues("controllerRegistrationName", registrationName)

		// Sometimes an operator needs to migrate to a new controller registration that supports the required
		// kind and types, but it is required to offboard the old extension. Thus, the operator marks the old
		// controller registration for deletion and manually delete its controller installation.
		// In parallel, Gardener should not create new controller installations for the deleted controller registration.
		if controllerRegistrations[registrationName].obj.DeletionTimestamp != nil {
			log.Info("Not creating or updating ControllerInstallation for ControllerRegistration because it is in deletion")
			continue
		}

		registrationLog.Info("Deploying wanted ControllerInstallation for ControllerRegistration")

		var (
			controllerDeployment   *gardencorev1.ControllerDeployment
			controllerRegistration = controllerRegistrations[registrationName].obj
		)

		if controllerRegistration.Spec.Deployment != nil && len(controllerRegistration.Spec.Deployment.DeploymentRefs) > 0 {
			// Today, only one DeploymentRef element is allowed, which is why can simply pick the first one from the slice.
			controllerDeployment = &gardencorev1.ControllerDeployment{}

			if err := c.Get(ctx, client.ObjectKey{Name: controllerRegistration.Spec.Deployment.DeploymentRefs[0].Name}, controllerDeployment); err != nil {
				return fmt.Errorf("cannot deploy ControllerInstallation because the referenced ControllerDeployment cannot be retrieved: %w", err)
			}
		}

		existingControllerInstallation := registrationNameToInstallation[registrationName]
		if existingControllerInstallation != nil && existingControllerInstallation.DeletionTimestamp != nil {
			return fmt.Errorf("cannot deploy new ControllerInstallation for %q because the deletion of the old ControllerInstallation is still pending", registrationName)
		}

		// When the seed created a ControllerInstallation exclusively (with SeedRefName label), but the shoot now
		// also needs it, hand over ownership by stripping the SeedRefName label. The CI already has .spec.seedRef
		// set, so the seed gardenlet will continue to see it.
		if existingControllerInstallation != nil && kind == SeedKind && selfHostedShoot != nil && metav1.HasLabel(existingControllerInstallation.ObjectMeta, SeedRefName) && shootNeededRegistrationNames.Has(registrationName) {
			registrationLog.Info("Handing over ControllerInstallation to shoot reconciler", "controllerInstallationName", existingControllerInstallation.Name)
			patch := client.MergeFrom(existingControllerInstallation.DeepCopy())
			delete(existingControllerInstallation.Labels, SeedRefName)
			if err := c.Patch(ctx, existingControllerInstallation, patch); err != nil {
				return fmt.Errorf("failed to strip SeedRefName label for ControllerInstallation %q: %w", existingControllerInstallation.Name, err)
			}
			continue
		}

		// When the shoot also needs this registration but no ControllerInstallation exists yet, skip creation —
		// the shoot reconciler will create it and the seed reconciler will adopt it on the next reconcile
		// (triggered by the ControllerInstallation watch).
		if existingControllerInstallation == nil && kind == SeedKind && selfHostedShoot != nil && shootNeededRegistrationNames.Has(registrationName) {
			registrationLog.Info("Skipping ControllerInstallation creation because the shoot reconciler will create it")
			continue
		}

		// When the seed reconciler finds an existing shoot-owned ControllerInstallation (no SeedRefName label) for the
		// same registration, the registration is already installed — skip it to avoid overwriting shoot-reconciler
		// ownership with the SeedRefName label. However, ensure `.spec.seedRef` is set so the seed gardenlet can see it.
		if existingControllerInstallation != nil && kind == SeedKind && selfHostedShoot != nil && !metav1.HasLabel(existingControllerInstallation.ObjectMeta, SeedRefName) {
			if existingControllerInstallation.Spec.SeedRef == nil {
				patch := client.MergeFrom(existingControllerInstallation.DeepCopy())
				existingControllerInstallation.Spec.SeedRef = &corev1.ObjectReference{Name: obj.GetName()}
				if err := c.Patch(ctx, existingControllerInstallation, patch); err != nil {
					return fmt.Errorf("failed to set .spec.seedRef for existing ControllerInstallation %q: %w", existingControllerInstallation.Name, err)
				}
			}
			registrationLog.Info("Skipping ControllerInstallation for ControllerRegistration because it is already managed by the shoot reconciler", "controllerInstallationName", existingControllerInstallation.Name)
			continue
		}

		if err := deployNeededInstallation(ctx, c, obj, kind, selfHostedShoot, controllerDeployment, controllerRegistration, existingControllerInstallation); err != nil {
			return err
		}
	}

	return nil
}

func deployNeededInstallation(
	ctx context.Context,
	c client.Client,
	obj client.Object,
	kind Kind,
	selfHostedShoot *gardencorev1beta1.Shoot,
	controllerDeployment *gardencorev1.ControllerDeployment,
	controllerRegistration *gardencorev1beta1.ControllerRegistration,
	existingControllerInstallation *gardencorev1beta1.ControllerInstallation,
) error {
	installationSpec := gardencorev1beta1.ControllerInstallationSpec{
		RegistrationRef: corev1.ObjectReference{
			Name:            controllerRegistration.Name,
			ResourceVersion: controllerRegistration.ResourceVersion,
		},
	}

	switch kind {
	case SeedKind:
		if selfHostedShoot != nil {
			// For self-hosted-shoot seeds the seed reconciler creates ControllerInstallations with both
			// `.spec.seedRef` (so that the seed gardenlet's cache can see them) and `.spec.shootRef` (so that the
			// shoot gardenlet can manage deployment).
			installationSpec.SeedRef = &corev1.ObjectReference{
				Name:            obj.GetName(),
				ResourceVersion: obj.GetResourceVersion(),
			}
			installationSpec.ShootRef = &corev1.ObjectReference{
				Name:            selfHostedShoot.Name,
				Namespace:       selfHostedShoot.Namespace,
				ResourceVersion: selfHostedShoot.ResourceVersion,
			}
		} else {
			installationSpec.SeedRef = &corev1.ObjectReference{
				Name:            obj.GetName(),
				ResourceVersion: obj.GetResourceVersion(),
			}
		}
	case ShootKind:
		installationSpec.ShootRef = &corev1.ObjectReference{
			Name:            obj.GetName(),
			Namespace:       obj.GetNamespace(),
			ResourceVersion: obj.GetResourceVersion(),
		}
	}

	if controllerDeployment != nil {
		installationSpec.DeploymentRef = &corev1.ObjectReference{
			Name:            controllerDeployment.Name,
			ResourceVersion: controllerDeployment.ResourceVersion,
		}
	}

	controllerInstallation := &gardencorev1beta1.ControllerInstallation{}
	mutate := func() error {
		switch kind {
		case SeedKind:
			if selfHostedShoot != nil {
				// Mark this ControllerInstallation as owned by the seed reconciler so that the shoot reconciler
				// does not accidentally manage or delete it (both use .spec.shootRef for the same shoot).
				metav1.SetMetaDataLabel(&controllerInstallation.ObjectMeta, SeedRefName, obj.GetName())

				shootSpecMap, err := convertObjToMap(selfHostedShoot.Spec)
				if err != nil {
					return err
				}
				shootSpecHash := utils.HashForMap(shootSpecMap)[:16]
				metav1.SetMetaDataLabel(&controllerInstallation.ObjectMeta, ShootSpecHash, shootSpecHash)
			} else {
				seed, ok := obj.(*gardencorev1beta1.Seed)
				if !ok {
					return fmt.Errorf("cannot convert object of type %T to *gardencorev1beta1.Seed", obj)
				}
				seedSpecMap, err := convertObjToMap(seed.Spec)
				if err != nil {
					return err
				}
				seedSpecHash := utils.HashForMap(seedSpecMap)[:16]
				metav1.SetMetaDataLabel(&controllerInstallation.ObjectMeta, SeedSpecHash, seedSpecHash)
			}

		case ShootKind:
			// If this ControllerInstallation was previously created by the seed reconciler for a self-hosted-shoot
			// seed, strip the ownership label to adopt it.
			delete(controllerInstallation.Labels, SeedRefName)

			shoot, ok := obj.(*gardencorev1beta1.Shoot)
			if !ok {
				return fmt.Errorf("cannot convert object of type %T to *gardencorev1beta1.Shoot", obj)
			}
			shootSpecMap, err := convertObjToMap(shoot.Spec)
			if err != nil {
				return err
			}
			shootSpecHash := utils.HashForMap(shootSpecMap)[:16]
			metav1.SetMetaDataLabel(&controllerInstallation.ObjectMeta, ShootSpecHash, shootSpecHash)
		}

		registrationSpecMap, err := convertObjToMap(controllerRegistration.Spec)
		if err != nil {
			return err
		}
		registrationSpecHash := utils.HashForMap(registrationSpecMap)[:16]
		metav1.SetMetaDataLabel(&controllerInstallation.ObjectMeta, RegistrationSpecHash, registrationSpecHash)

		if controllerDeployment != nil {
			// Add all fields that are relevant for the hash calculation as `ControllerDeployment`s don't have a `spec` field.
			hashFields := map[string]any{
				"type":           controllerDeployment.Annotations[gardencorev1.MigrationControllerDeploymentType],
				"providerConfig": controllerDeployment.Annotations[gardencorev1.MigrationControllerDeploymentProviderConfig],
				"helm":           controllerDeployment.Helm,
			}

			deploymentMap, err := convertObjToMap(hashFields)
			if err != nil {
				return err
			}
			deploymentSpecHash := utils.HashForMap(deploymentMap)[:16]
			metav1.SetMetaDataLabel(&controllerInstallation.ObjectMeta, ControllerDeploymentHash, deploymentSpecHash)
		}

		if podSecurityEnforce, ok := controllerRegistration.Annotations[v1beta1constants.AnnotationPodSecurityEnforce]; ok {
			metav1.SetMetaDataAnnotation(&controllerInstallation.ObjectMeta, v1beta1constants.AnnotationPodSecurityEnforce, podSecurityEnforce)
		} else {
			delete(controllerInstallation.Annotations, v1beta1constants.AnnotationPodSecurityEnforce)
		}

		// When the shoot reconciler updates an existing ControllerInstallation that was previously created by the
		// seed reconciler (with .spec.seedRef set), preserve .spec.seedRef so the seed gardenlet can still see it.
		existingSeedRef := controllerInstallation.Spec.SeedRef
		controllerInstallation.Spec = installationSpec
		if kind == ShootKind && controllerInstallation.Spec.SeedRef == nil && existingSeedRef != nil {
			controllerInstallation.Spec.SeedRef = existingSeedRef
		}
		return nil
	}

	if existingControllerInstallation != nil {
		// The installation already exists, however, we do not have the latest version of the ControllerInstallation object.
		// Hence, we are running the `GetAndCreateOrMergePatch` function as it first GETs the current objects and then runs the
		// mutate() func before sending the PATCH. This way we ensure that we have applied our mutations to the latest version.
		controllerInstallation.Name = existingControllerInstallation.Name
		_, err := controllerutils.GetAndCreateOrMergePatch(ctx, c, controllerInstallation, mutate)
		return err
	}

	// The installation does not exist yet, hence, we set `GenerateName` which will automatically append a random suffix to
	// the name. Unfortunately, the `CreateOrUpdate` function does not support creating an object that does not specify a name
	// but only `GenerateName`, thus, we call `Create` directly.
	controllerInstallation.GenerateName = controllerRegistration.Name + "-"
	_ = mutate()
	return c.Create(ctx, controllerInstallation)
}

// deleteUnneededInstallations takes the list of required names of ControllerRegistrations, and another mapping of
// ControllerRegistration names to existing ControllerInstallations. It deletes every existing ControllerInstallation
// whose referenced ControllerRegistration is not part of the given list of required list.
// For the seed reconciler of self-hosted shoot seeds:
//   - Seed-owned CIs (with SeedRefName label) that are still needed by the shoot reconciler are handed over by
//     stripping the label.
//   - Seed-owned CIs (with SeedRefName label) that are not needed by the shoot are deleted directly.
//   - Shoot-owned CIs (no SeedRefName label) have `.spec.seedRef` cleared so the seed gardenlet no longer sees them.
//
// For the seed reconciler of regular seeds:
// - Seed-owned CIs (with SeedRefName label) are deleted directly.
// For the shoot reconciler: ControllerInstallations with the SeedRefName label are skipped entirely (the seed
// reconciler owns them).
func deleteUnneededInstallations(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	kind Kind,
	selfHostedShoot *gardencorev1beta1.Shoot,
	wantedControllerRegistrationNames sets.Set[string],
	shootNeededRegistrationNames sets.Set[string],
	registrationNameToInstallation map[string]*gardencorev1beta1.ControllerInstallation,
) error {
	for registrationName, installation := range registrationNameToInstallation {
		if wantedControllerRegistrationNames.Has(registrationName) {
			continue
		}

		// ControllerInstallations with the SeedRefName label are owned by the seed reconciler.
		if metav1.HasLabel(installation.ObjectMeta, SeedRefName) {
			if kind == ShootKind {
				// The shoot reconciler must not delete or modify seed-owned ControllerInstallations.
				continue
			}

			if selfHostedShoot != nil && shootNeededRegistrationNames.Has(registrationName) {
				// The shoot reconciler still needs this ControllerInstallation. Hand over ownership by
				// stripping the SeedRefName label so the shoot reconciler takes over.
				log.Info("Handing over ControllerInstallation to shoot reconciler", "controllerRegistrationName", registrationName, "controllerInstallationName", installation.Name)
				patch := client.MergeFrom(installation.DeepCopy())
				delete(installation.Labels, SeedRefName)
				if err := c.Patch(ctx, installation, patch); err != nil {
					return err
				}
				continue
			}

			log.Info("Deleting ControllerInstallation for ControllerRegistration no longer needed by seed", "controllerRegistrationName", registrationName, "controllerInstallationName", installation.Name)
			if err := c.Delete(ctx, installation); client.IgnoreNotFound(err) != nil {
				return err
			}
			continue
		}

		if kind == SeedKind && selfHostedShoot != nil {
			// The shoot still needs this CI. Clear `.spec.seedRef` so the seed gardenlet no longer sees it.
			if installation.Spec.SeedRef != nil {
				patch := client.MergeFrom(installation.DeepCopy())
				installation.Spec.SeedRef = nil
				if err := c.Patch(ctx, installation, patch); err != nil {
					return err
				}
			}
			continue
		}

		log.Info("Deleting unneeded ControllerInstallation for ControllerRegistration", "controllerRegistrationName", registrationName, "controllerInstallationName", installation.Name)
		if err := c.Delete(ctx, installation); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	return nil
}

func registrationNamesForKindTypes(kindTypes sets.Set[string], controllerRegistrations map[string]controllerRegistration) sets.Set[string] {
	names := sets.New[string]()
	for name, reg := range controllerRegistrations {
		for _, resource := range reg.obj.Spec.Resources {
			if kindTypes.Has(gardenerutils.ExtensionsID(resource.Kind, resource.Type)) {
				names.Insert(name)
				break
			}
		}
	}
	return names
}

func convertObjToMap(in any) (map[string]any, error) {
	var out map[string]any

	data, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}

	return out, nil
}

func seedSelectorMatches(deployment *gardencorev1beta1.ControllerRegistrationDeployment, seedLabels map[string]string) (bool, error) {
	selector := &metav1.LabelSelector{}
	if deployment != nil && deployment.SeedSelector != nil {
		selector = deployment.SeedSelector
	}

	seedSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return false, fmt.Errorf("label selector conversion failed: %v for seedSelector: %w", *selector, err)
	}

	return seedSelector.Matches(labels.Set(seedLabels)), nil
}

func controllerRegistrationNamesWithMatchingSeedLabelSelector(
	namesInQuestion []string,
	controllerRegistrations map[string]controllerRegistration,
	seedLabels map[string]string,
) (sets.Set[string], error) {
	matchingNames := sets.New[string]()

	for _, name := range namesInQuestion {
		controllerRegistration, ok := controllerRegistrations[name]
		if !ok {
			return nil, fmt.Errorf("ControllerRegistration with name %s not found", name)
		}

		matches, err := seedSelectorMatches(controllerRegistration.obj.Spec.Deployment, seedLabels)
		if err != nil {
			return nil, err
		}

		if matches {
			matchingNames.Insert(name)
		}
	}

	return matchingNames, nil
}

func getShoots(ctx context.Context, c client.Reader, obj client.Object, kind Kind) ([]gardencorev1beta1.Shoot, error) {
	if kind == ShootKind {
		shoot, ok := obj.(*gardencorev1beta1.Shoot)
		if !ok {
			return nil, fmt.Errorf("cannot convert object of type %T to *gardencorev1beta1.Shoot", obj)
		}
		return []gardencorev1beta1.Shoot{*shoot}, nil
	}

	shootList := &gardencorev1beta1.ShootList{}
	if err := c.List(ctx, shootList, client.MatchingFields{core.ShootSeedName: obj.GetName()}); err != nil {
		return nil, err
	}
	shootListAsItems := v1beta1helper.ShootItems(*shootList)

	shootList2 := &gardencorev1beta1.ShootList{}
	if err := c.List(ctx, shootList2, client.MatchingFields{core.ShootStatusSeedName: obj.GetName()}); err != nil {
		return nil, err
	}
	shootListAsItems2 := v1beta1helper.ShootItems(*shootList2)

	return shootListAsItems.Union(&shootListAsItems2), nil
}

func controllerInstallationReferencesObject(controllerInstallation gardencorev1beta1.ControllerInstallation, obj client.Object, kind Kind, selfHostedShoot *gardencorev1beta1.Shoot) bool {
	switch kind {
	case SeedKind:
		if selfHostedShoot != nil {
			// For self-hosted-shoot seeds, match all ControllerInstallations with .spec.shootRef for this shoot —
			// both seed-owned (with SeedRefName label) and shoot-owned (without). This allows the seed reconciler to
			// discover and adopt existing shoot-owned installations instead of creating duplicates.
			if controllerInstallation.Spec.ShootRef == nil ||
				controllerInstallation.Spec.ShootRef.Name != selfHostedShoot.Name ||
				controllerInstallation.Spec.ShootRef.Namespace != selfHostedShoot.Namespace {
				return false
			}
		} else {
			if controllerInstallation.Spec.SeedRef == nil || controllerInstallation.Spec.SeedRef.Name != obj.GetName() {
				return false
			}
		}
	case ShootKind:
		if controllerInstallation.Spec.ShootRef == nil ||
			controllerInstallation.Spec.ShootRef.Name != obj.GetName() ||
			controllerInstallation.Spec.ShootRef.Namespace != obj.GetNamespace() {
			return false
		}
	}

	return true
}

func backupBucketFieldSelector(obj client.Object, kind Kind) []client.ListOption {
	switch kind {
	case ShootKind:
		return []client.ListOption{client.MatchingFields{core.BackupBucketShootRefName: obj.GetName(), core.BackupBucketShootRefNamespace: obj.GetNamespace()}}
	default:
		return nil
	}
}

func backupEntryFieldSelector(obj client.Object, kind Kind) []client.ListOption {
	switch kind {
	case SeedKind:
		return []client.ListOption{client.MatchingFields{core.BackupEntrySeedName: obj.GetName()}}
	case ShootKind:
		return []client.ListOption{client.MatchingFields{core.BackupEntryShootRefName: obj.GetName(), core.BackupEntryShootRefNamespace: obj.GetNamespace()}}
	default:
		return nil
	}
}
