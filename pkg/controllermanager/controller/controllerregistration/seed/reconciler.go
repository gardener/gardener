// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

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

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	// SeedSpecHash is a constant for a label on `ControllerInstallation`s (similar to `pod-template-hash` on `Pod`s).
	SeedSpecHash = "seed-spec-hash"
	// ControllerDeploymentHash is a constant for a label on `ControllerInstallation`s (similar to `pod-template-hash` on `Pod`s).
	ControllerDeploymentHash = "deployment-hash"
	// RegistrationSpecHash is a constant for a label on `ControllerInstallation`s (similar to `pod-template-hash` on `Pod`s).
	RegistrationSpecHash = "registration-spec-hash"
)

// Reconciler determines which ControllerRegistrations are required for a given Seed by checking all objects in the
// garden cluster, that need to be considered for that Seed (e.g. because they reference the Seed). It then deploys
// wanted and deletes unneeded ControllerInstallations accordingly. Seeds get enqueued by updates to relevant
// (referencing) objects, e.g. Shoots, BackupBuckets, etc. This is the main reconciler of this controller, that does the
// actual work.
type Reconciler struct {
	Client    client.Client
	APIReader client.Reader
	Config    controllermanagerconfigv1alpha1.ControllerRegistrationControllerConfiguration
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	seed := &gardencorev1beta1.Seed{}
	if err := r.Client.Get(ctx, request.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	log.Info("Reconciling Seed")

	controllerRegistrationList := &gardencorev1beta1.ControllerRegistrationList{}
	if err := r.Client.List(ctx, controllerRegistrationList); err != nil {
		return reconcile.Result{}, err
	}

	// Live lookup to prevent working on a stale cache and trying to create multiple installations for the same
	// registration/seed combination.
	controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
	if err := r.APIReader.List(ctx, controllerInstallationList); err != nil {
		return reconcile.Result{}, err
	}

	backupBucketList := &gardencorev1beta1.BackupBucketList{}
	if err := r.Client.List(ctx, backupBucketList); err != nil {
		return reconcile.Result{}, err
	}
	backupBucketNameToObject := make(map[string]*gardencorev1beta1.BackupBucket, len(backupBucketList.Items))
	for _, backupBucket := range backupBucketList.Items {
		backupBucketNameToObject[backupBucket.Name] = &backupBucket
	}

	backupEntryList := &gardencorev1beta1.BackupEntryList{}
	if err := r.APIReader.List(ctx, backupEntryList, client.MatchingFields{core.BackupEntrySeedName: seed.Name}); err != nil {
		return reconcile.Result{}, err
	}

	shootList, err := getShoots(ctx, r.APIReader, seed)
	if err != nil {
		return reconcile.Result{}, err
	}

	secrets, err := gardenerutils.ReadGardenSecrets(ctx, log, r.Client, gardenerutils.ComputeGardenNamespace(seed.Name), false)
	if err != nil {
		return reconcile.Result{}, err
	}

	if len(secrets) < 1 {
		return reconcile.Result{}, fmt.Errorf("garden secrets for seed %q have not been synchronized yet", seed.Name)
	}

	internalDomain, err := gardenerutils.GetInternalDomain(secrets)
	if err != nil {
		return reconcile.Result{}, err
	}
	defaultDomains, err := gardenerutils.GetDefaultDomains(secrets)
	if err != nil {
		return reconcile.Result{}, err
	}

	var (
		controllerRegistrations = computeControllerRegistrationMaps(controllerRegistrationList)

		wantedKindTypeCombinationForBackupBuckets = computeKindTypesForBackupBuckets(backupBucketNameToObject, seed.Name)
		wantedKindTypeCombinationForBackupEntries = computeKindTypesForBackupEntries(log, backupEntryList, backupBucketNameToObject)
		wantedKindTypeCombinationForShoots        = computeKindTypesForShoots(ctx, log, r.Client, shootList, seed, controllerRegistrationList, internalDomain, defaultDomains)
		wantedKindTypeCombinationForSeed          = computeKindTypesForSeed(seed, controllerRegistrationList)

		wantedKindTypeCombinations = sets.
						New[string]().
						Union(wantedKindTypeCombinationForBackupBuckets).
						Union(wantedKindTypeCombinationForBackupEntries).
						Union(wantedKindTypeCombinationForShoots).
						Union(wantedKindTypeCombinationForSeed)
	)

	wantedControllerRegistrationNames, err := computeWantedControllerRegistrationNames(wantedKindTypeCombinations, controllerInstallationList, controllerRegistrations, len(shootList), seed.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}

	registrationNameToInstallation, err := computeRegistrationNameToInstallationMap(controllerInstallationList, controllerRegistrations, seed.Name)
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := deployNeededInstallations(ctx, log, r.Client, seed, wantedControllerRegistrationNames, controllerRegistrations, registrationNameToInstallation); err != nil {
		return reconcile.Result{}, err
	}

	if err := deleteUnneededInstallations(ctx, log, r.Client, wantedControllerRegistrationNames, registrationNameToInstallation); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// computeKindTypesForBackupBucket computes the list of wanted kind/type combinations for extension resources based on the
// the list of existing BackupBucket resources.
func computeKindTypesForBackupBuckets(
	backupBucketNameToObject map[string]*gardencorev1beta1.BackupBucket,
	seedName string,
) sets.Set[string] {
	wantedKindTypeCombinations := sets.New[string]()

	for _, backupBucket := range backupBucketNameToObject {
		if ptr.Deref(backupBucket.Spec.SeedName, "") == seedName {
			wantedKindTypeCombinations.Insert(gardenerutils.ExtensionsID(extensionsv1alpha1.BackupBucketResource, backupBucket.Spec.Provider.Type))
		}
	}

	return wantedKindTypeCombinations
}

// computeKindTypesForBackupEntries computes the list of wanted kind/type combinations for extension resources based on the
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

// computeKindTypesForShoots computes the list of wanted kind/type combinations for extension resources based on the
// the list of existing Shoot resources.
func computeKindTypesForShoots(
	ctx context.Context,
	log logr.Logger,
	c client.Reader,
	shootList []gardencorev1beta1.Shoot,
	seed *gardencorev1beta1.Seed,
	controllerRegistrationList *gardencorev1beta1.ControllerRegistrationList,
	internalDomain *gardenerutils.Domain,
	defaultDomains []*gardenerutils.Domain,
) sets.Set[string] {
	var (
		wantedKindTypeCombinations = sets.New[string]()

		wg  sync.WaitGroup
		out = make(chan sets.Set[string])
	)

	for _, shoot := range shootList {
		if (shoot.Spec.SeedName == nil || *shoot.Spec.SeedName != seed.Name) && (shoot.Status.SeedName == nil || *shoot.Status.SeedName != seed.Name) {
			continue
		}

		wg.Add(1)
		go func(shoot *gardencorev1beta1.Shoot) {
			defer wg.Done()

			externalDomain, err := gardenerutils.ConstructExternalDomain(ctx, c, shoot, &corev1.Secret{}, defaultDomains)
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

	return wantedKindTypeCombinations
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

// computeWantedControllerRegistrationNames computes the list of names of ControllerRegistration objects that are desired
// to be installed. The computation is performed based on a list of required kind/type combinations and the proper mapping
// to existing ControllerRegistration objects. Additionally, all names in the alwaysPolicyControllerRegistrationNames list
// will be returned and all currently installed and required installations.
func computeWantedControllerRegistrationNames(
	wantedKindTypeCombinations sets.Set[string],
	controllerInstallationList *gardencorev1beta1.ControllerInstallationList,
	controllerRegistrations map[string]controllerRegistration,
	numberOfShoots int,
	seedObjectMeta metav1.ObjectMeta,
) (
	sets.Set[string],
	error,
) {
	var (
		kindTypeToControllerRegistrationNames = make(map[string][]string)
		wantedControllerRegistrationNames     = sets.New[string]()
	)

	for name, controllerRegistration := range controllerRegistrations {
		if controllerRegistration.obj.DeletionTimestamp == nil {
			if controllerRegistration.deployAlways && seedObjectMeta.DeletionTimestamp == nil {
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

	wantedControllerRegistrationNames.Insert(sets.List(installedAndRequiredRegistrationNames(controllerInstallationList, seedObjectMeta.Name))...)

	// filter controller registrations with non-matching seed selector
	return controllerRegistrationNamesWithMatchingSeedLabelSelector(wantedControllerRegistrationNames.UnsortedList(), controllerRegistrations, seedObjectMeta.Labels)
}

func installedAndRequiredRegistrationNames(controllerInstallationList *gardencorev1beta1.ControllerInstallationList, seedName string) sets.Set[string] {
	requiredControllerRegistrationNames := sets.New[string]()
	for _, controllerInstallation := range controllerInstallationList.Items {
		if controllerInstallation.Spec.SeedRef.Name != seedName {
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
	seedName string,
) (
	map[string]*gardencorev1beta1.ControllerInstallation,
	error,
) {
	registrationNameToInstallationName := make(map[string]*gardencorev1beta1.ControllerInstallation)

	for _, controllerInstallation := range controllerInstallationList.Items {
		if controllerInstallation.Spec.SeedRef.Name != seedName {
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
// creates or update ControllerInstallation objects for that reference the given seed and the various desired ControllerRegistrations.
func deployNeededInstallations(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	seed *gardencorev1beta1.Seed,
	wantedControllerRegistrations sets.Set[string],
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

		if err := deployNeededInstallation(ctx, c, seed, controllerDeployment, controllerRegistration, existingControllerInstallation); err != nil {
			return err
		}
	}

	return nil
}

func deployNeededInstallation(
	ctx context.Context,
	c client.Client,
	seed *gardencorev1beta1.Seed,
	controllerDeployment *gardencorev1.ControllerDeployment,
	controllerRegistration *gardencorev1beta1.ControllerRegistration,
	existingControllerInstallation *gardencorev1beta1.ControllerInstallation,
) error {
	installationSpec := gardencorev1beta1.ControllerInstallationSpec{
		SeedRef: corev1.ObjectReference{
			Name:            seed.Name,
			ResourceVersion: seed.ResourceVersion,
		},
		RegistrationRef: corev1.ObjectReference{
			Name:            controllerRegistration.Name,
			ResourceVersion: controllerRegistration.ResourceVersion,
		},
	}

	if controllerDeployment != nil {
		installationSpec.DeploymentRef = &corev1.ObjectReference{
			Name:            controllerDeployment.Name,
			ResourceVersion: controllerDeployment.ResourceVersion,
		}
	}

	controllerInstallation := &gardencorev1beta1.ControllerInstallation{}
	mutate := func() error {
		seedSpecMap, err := convertObjToMap(seed.Spec)
		if err != nil {
			return err
		}
		seedSpecHash := utils.HashForMap(seedSpecMap)[:16]
		metav1.SetMetaDataLabel(&controllerInstallation.ObjectMeta, SeedSpecHash, seedSpecHash)

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

		controllerInstallation.Spec = installationSpec
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
// ControllerRegistration names to existing ControllerInstallations. It deletes every existing ControllerInstallation whose
// referenced ControllerRegistration is not part of the given list of required list.
func deleteUnneededInstallations(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	wantedControllerRegistrationNames sets.Set[string],
	registrationNameToInstallation map[string]*gardencorev1beta1.ControllerInstallation,
) error {
	for registrationName, installation := range registrationNameToInstallation {
		if !wantedControllerRegistrationNames.Has(registrationName) {
			log.Info("Deleting unneeded ControllerInstallation for ControllerRegistration", "controllerRegistrationName", registrationName, "controllerInstallationName", installation.Name)

			if err := c.Delete(ctx, installation); client.IgnoreNotFound(err) != nil {
				return err
			}
		}
	}

	return nil
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

func getShoots(ctx context.Context, c client.Reader, seed *gardencorev1beta1.Seed) ([]gardencorev1beta1.Shoot, error) {
	shootList := &gardencorev1beta1.ShootList{}
	if err := c.List(ctx, shootList, client.MatchingFields{core.ShootSeedName: seed.Name}); err != nil {
		return nil, err
	}
	shootListAsItems := v1beta1helper.ShootItems(*shootList)

	shootList2 := &gardencorev1beta1.ShootList{}
	if err := c.List(ctx, shootList2, client.MatchingFields{core.ShootStatusSeedName: seed.Name}); err != nil {
		return nil, err
	}
	shootListAsItems2 := v1beta1helper.ShootItems(*shootList2)

	return shootListAsItems.Union(&shootListAsItems2), nil
}
