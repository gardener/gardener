// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerregistration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	gardenpkg "github.com/gardener/gardener/pkg/operation/garden"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (c *Controller) reconcileControllerRegistrationSeedKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	seed, err := c.seedLister.Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[CONTROLLERREGISTRATION SEED RECONCILE] %s - skipping because Seed has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[CONTROLLERREGISTRATION SEED RECONCILE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.controllerRegistrationSeedControl.Reconcile(seed)
}

// RegistrationSeedControlInterface implements the control logic for updating Seeds. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type RegistrationSeedControlInterface interface {
	Reconcile(*gardencorev1beta1.Seed) error
}

// NewDefaultControllerRegistrationSeedControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for Seeds. You should use an instance returned from NewDefaultControllerRegistrationSeedControl()
// for any scenario other than testing.
func NewDefaultControllerRegistrationSeedControl(
	clientMap clientmap.ClientMap,
	secrets map[string]*corev1.Secret,
	backupBucketLister gardencorelisters.BackupBucketLister,
	controllerInstallationLister gardencorelisters.ControllerInstallationLister,
	controllerRegistrationLister gardencorelisters.ControllerRegistrationLister,
	seedLister gardencorelisters.SeedLister,
) RegistrationSeedControlInterface {
	return &defaultControllerRegistrationSeedControl{clientMap, secrets, backupBucketLister, controllerInstallationLister, controllerRegistrationLister, seedLister}
}

type defaultControllerRegistrationSeedControl struct {
	clientMap                    clientmap.ClientMap
	secrets                      map[string]*corev1.Secret
	backupBucketLister           gardencorelisters.BackupBucketLister
	controllerInstallationLister gardencorelisters.ControllerInstallationLister
	controllerRegistrationLister gardencorelisters.ControllerRegistrationLister
	seedLister                   gardencorelisters.SeedLister
}

// Reconcile reconciles the ControllerInstallations for given Seed object. It computes the desired list of
// ControllerInstallations, deploys them, and deletes all other potentially existing installation objects.
func (c *defaultControllerRegistrationSeedControl) Reconcile(obj *gardencorev1beta1.Seed) error {
	var (
		ctx    = context.TODO()
		seed   = obj.DeepCopy()
		logger = logger.NewFieldLogger(logger.Logger, "controllerregistration-seed", seed.Name)
	)

	logger.Infof("[CONTROLLERINSTALLATION SEED] Reconciling %s", seed.Name)

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	controllerRegistrationList, err := c.controllerRegistrationLister.List(labels.Everything())
	if err != nil {
		return err
	}
	// Live lookup to prevent working on a stale cache and trying to create multiple installations for the same
	// registration/seed combination.
	controllerInstallationList, err := gardenClient.GardenCore().CoreV1beta1().ControllerInstallations().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	backupBucketList, err := c.backupBucketLister.List(labels.Everything())
	if err != nil {
		return err
	}
	backupEntryList, err := gardenClient.GardenCore().CoreV1beta1().BackupEntries(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{core.BackupEntrySeedName: seed.Name}).String(),
	})
	if err != nil {
		return err
	}

	shootList, err := getShoots(ctx, gardenClient.GardenCore(), seed)
	if err != nil {
		return err
	}

	internalDomain, err := gardenpkg.GetInternalDomain(c.secrets)
	if err != nil {
		return err
	}
	defaultDomains, err := gardenpkg.GetDefaultDomains(c.secrets)
	if err != nil {
		return err
	}

	var (
		controllerRegistrations = computeControllerRegistrationMaps(controllerRegistrationList)

		wantedKindTypeCombinationForBackupBuckets, buckets = computeKindTypesForBackupBuckets(backupBucketList, seed.Name)
		wantedKindTypeCombinationForBackupEntries          = computeKindTypesForBackupEntries(logger, backupEntryList, buckets, seed.Name)
		wantedKindTypeCombinationForShoots                 = computeKindTypesForShoots(ctx, logger, gardenClient.Client(), shootList, seed, controllerRegistrationList, internalDomain, defaultDomains)

		wantedKindTypeCombinations = sets.
						NewString().
						Union(wantedKindTypeCombinationForBackupBuckets).
						Union(wantedKindTypeCombinationForBackupEntries).
						Union(wantedKindTypeCombinationForShoots)
	)

	wantedControllerRegistrationNames, err := computeWantedControllerRegistrationNames(wantedKindTypeCombinations, controllerInstallationList, controllerRegistrations, seed.Labels, seed.Name)
	if err != nil {
		return err
	}

	registrationNameToInstallationName, err := computeRegistrationNameToInstallationNameMap(controllerInstallationList, controllerRegistrations, seed.Name)
	if err != nil {
		return err
	}

	if err := deployNeededInstallations(ctx, logger, gardenClient.Client(), seed, wantedControllerRegistrationNames, controllerRegistrations, registrationNameToInstallationName); err != nil {
		return err
	}
	return deleteUnneededInstallations(ctx, logger, gardenClient.Client(), wantedControllerRegistrationNames, registrationNameToInstallationName)
}

// computeKindTypesForBackupBucket computes the list of wanted kind/type combinations for extension resources based on the
// the list of existing BackupBucket resources.
func computeKindTypesForBackupBuckets(
	backupBucketList []*gardencorev1beta1.BackupBucket,
	seedName string,
) (sets.String, map[string]*gardencorev1beta1.BackupBucket) {
	var (
		wantedKindTypeCombinations = sets.NewString()
		buckets                    = make(map[string]*gardencorev1beta1.BackupBucket)
	)

	for _, backupBucket := range backupBucketList {
		buckets[backupBucket.Name] = backupBucket

		if backupBucket.Spec.SeedName == nil || *backupBucket.Spec.SeedName != seedName {
			continue
		}

		wantedKindTypeCombinations.Insert(common.ExtensionID(extensionsv1alpha1.BackupBucketResource, backupBucket.Spec.Provider.Type))
	}

	return wantedKindTypeCombinations, buckets
}

// computeKindTypesForBackupEntries computes the list of wanted kind/type combinations for extension resources based on the
// the list of existing BackupEntry resources.
func computeKindTypesForBackupEntries(
	logger *logrus.Entry,
	backupEntryList *gardencorev1beta1.BackupEntryList,
	buckets map[string]*gardencorev1beta1.BackupBucket,
	seedName string,
) sets.String {
	wantedKindTypeCombinations := sets.NewString()

	for _, backupEntry := range backupEntryList.Items {
		if backupEntry.Spec.SeedName == nil || *backupEntry.Spec.SeedName != seedName {
			continue
		}

		bucket, ok := buckets[backupEntry.Spec.BucketName]
		if !ok {
			logger.Errorf("couldn't find BackupBucket %q for BackupEntry %q", backupEntry.Spec.BucketName, backupEntry.Name)
			continue
		}

		wantedKindTypeCombinations.Insert(common.ExtensionID(extensionsv1alpha1.BackupEntryResource, bucket.Spec.Provider.Type))
	}

	return wantedKindTypeCombinations
}

// computeKindTypesForShoots computes the list of wanted kind/type combinations for extension resources based on the
// the list of existing Shoot resources.
func computeKindTypesForShoots(
	ctx context.Context,
	logger *logrus.Entry,
	client client.Client,
	shootList []gardencorev1beta1.Shoot,
	seed *gardencorev1beta1.Seed,
	controllerRegistrationList []*gardencorev1beta1.ControllerRegistration,
	internalDomain *gardenpkg.Domain,
	defaultDomains []*gardenpkg.Domain,
) sets.String {
	var (
		wantedKindTypeCombinations = sets.NewString()

		wg  sync.WaitGroup
		out = make(chan sets.String)
	)

	for _, shoot := range shootList {
		if (shoot.Spec.SeedName == nil || *shoot.Spec.SeedName != seed.Name) && (shoot.Status.SeedName == nil || *shoot.Status.SeedName != seed.Name) {
			continue
		}

		wg.Add(1)
		go func(shoot *gardencorev1beta1.Shoot) {
			defer wg.Done()

			externalDomain, err := shootpkg.ConstructExternalDomain(ctx, client, shoot, &corev1.Secret{}, defaultDomains)
			if err != nil && !(shootpkg.IsIncompleteDNSConfigError(err) && shoot.DeletionTimestamp != nil && len(shoot.Status.UID) == 0) {
				logger.Warnf("could not determine external domain for shoot %s/%s: %+v", shoot.Namespace, shoot.Name, err)
			}

			out <- shootpkg.ComputeRequiredExtensions(shoot, seed, controllerRegistrationList, internalDomain, externalDomain)
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

type controllerRegistration struct {
	obj          *gardencorev1beta1.ControllerRegistration
	deployAlways bool
}

// computeControllerRegistrationMaps computes two maps and a string slice. The first map maps the name of a
// ControllerRegistration to the *gardencorev1beta1.ControllerRegistration object. The second map maps the registered
// kind/type combinations to the supporting *gardencorev1beta1.ControllerRegistration objects. If the ControllerRegistration
// specifies a seed selector then it will be validated against the provided seed labels map. Only if it matches then the
// ControllerRegistration will be considered. The string slice contains the list of names of ControllerRegistrations
// having the 'Always' deployment policy.
func computeControllerRegistrationMaps(
	controllerRegistrationList []*gardencorev1beta1.ControllerRegistration,
) map[string]controllerRegistration {
	var out = make(map[string]controllerRegistration)
	for _, cr := range controllerRegistrationList {
		out[cr.Name] = controllerRegistration{
			obj:          cr.DeepCopy(),
			deployAlways: cr.Spec.Deployment != nil && cr.Spec.Deployment.Policy != nil && *cr.Spec.Deployment.Policy == gardencorev1beta1.ControllerDeploymentPolicyAlways,
		}
	}
	return out
}

// computeWantedControllerRegistrationNames computes the list of names of ControllerRegistration objects that are desired
// to be installed. The computation is performed based on a list of required kind/type combinations and the proper mapping
// to existing ControllerRegistration objects. Additionally, all names in the alwaysPolicyControllerRegistrationNames list
// will be returned.
func computeWantedControllerRegistrationNames(
	wantedKindTypeCombinations sets.String,
	controllerInstallationList *gardencorev1beta1.ControllerInstallationList,
	controllerRegistrations map[string]controllerRegistration,
	seedLabels map[string]string,
	seedName string,
) (sets.String, error) {
	var (
		kindTypeToControllerRegistrationNames = make(map[string][]string)
		wantedControllerRegistrationNames     = sets.NewString()
	)

	for name, controllerRegistration := range controllerRegistrations {
		if controllerRegistration.deployAlways {
			wantedControllerRegistrationNames.Insert(name)
		}

		for _, resource := range controllerRegistration.obj.Spec.Resources {
			id := common.ExtensionID(resource.Kind, resource.Type)
			kindTypeToControllerRegistrationNames[id] = append(kindTypeToControllerRegistrationNames[id], name)
		}
	}

	for _, requiredExtension := range wantedKindTypeCombinations.UnsortedList() {
		names, ok := kindTypeToControllerRegistrationNames[requiredExtension]
		if !ok {
			return nil, fmt.Errorf("need to install an extension controller for %q but no appropriate ControllerRegistration found", requiredExtension)
		}

		wantedControllerRegistrationNames.Insert(names...)
	}

	for _, controllerInstallation := range controllerInstallationList.Items {
		if controllerInstallation.Spec.SeedRef.Name != seedName {
			continue
		}
		if !gardencorev1beta1helper.IsControllerInstallationRequired(controllerInstallation) {
			continue
		}
		wantedControllerRegistrationNames.Insert(controllerInstallation.Spec.RegistrationRef.Name)
	}

	// filter controller registrations with non-matching seed selector
	return controllerRegistrationNamesWithMatchingSeedLabelSelector(wantedControllerRegistrationNames.UnsortedList(), controllerRegistrations, seedLabels)
}

// computeRegistrationNameToInstallationNameMap computes a map that maps the name of a ControllerRegistration to the name of an
// existing ControllerInstallation object that references this registration.
func computeRegistrationNameToInstallationNameMap(
	controllerInstallationList *gardencorev1beta1.ControllerInstallationList,
	controllerRegistrations map[string]controllerRegistration,
	seedName string,
) (map[string]string, error) {
	registrationNameToInstallationName := make(map[string]string)

	for _, controllerInstallation := range controllerInstallationList.Items {
		if controllerInstallation.Spec.SeedRef.Name != seedName {
			continue
		}

		if _, ok := controllerRegistrations[controllerInstallation.Spec.RegistrationRef.Name]; !ok {
			return nil, fmt.Errorf("ControllerRegistration %q does not exist", controllerInstallation.Spec.RegistrationRef.Name)
		}

		registrationNameToInstallationName[controllerInstallation.Spec.RegistrationRef.Name] = controllerInstallation.Name
	}

	return registrationNameToInstallationName, nil
}

// deployNeededInstallations takes the list of required names of ControllerRegistrations, a mapping of ControllerRegistration
// names to their actual objects, and another mapping of ControllerRegistration names to existing ControllerInstallations. It
// creates or update ControllerInstallation objects for that reference the given seed and the various desired ControllerRegistrations.
func deployNeededInstallations(
	ctx context.Context,
	logger *logrus.Entry,
	c client.Client,
	seed *gardencorev1beta1.Seed,
	wantedControllerRegistrations sets.String,
	controllerRegistrations map[string]controllerRegistration,
	registrationNameToInstallationName map[string]string,
) error {
	for _, registrationName := range wantedControllerRegistrations.UnsortedList() {
		// Sometimes an operator needs to migrate to a new controller registration that supports the required
		// kind and types, but it is required to offboard the old extension. Thus, the operator marks the old
		// controller registration for deletion and manually delete its controller installation.
		// In parallel, Gardener should not create new controller installations for the deleted controller registation.
		if controllerRegistrations[registrationName].obj.DeletionTimestamp != nil {
			logger.Infof("Do not create or update ControllerInstallation for %q which is in deletion", registrationName)
			continue
		}

		logger.Infof("Deploying wanted ControllerInstallation for %q", registrationName)

		if err := deployNeededInstallation(ctx, c, seed, controllerRegistrations[registrationName].obj, registrationNameToInstallationName[registrationName]); err != nil {
			return err
		}
	}

	return nil
}

func deployNeededInstallation(
	ctx context.Context,
	c client.Client,
	seed *gardencorev1beta1.Seed,
	controllerRegistration *gardencorev1beta1.ControllerRegistration,
	controllerInstallationName string,
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

	seedSpecMap, err := convertObjToMap(seed.Spec)
	if err != nil {
		return err
	}
	registrationSpecMap, err := convertObjToMap(controllerRegistration.Spec)
	if err != nil {
		return err
	}

	var (
		registrationSpecHash = utils.HashForMap(registrationSpecMap)[:16]
		seedSpecHash         = utils.HashForMap(seedSpecMap)[:16]

		controllerInstallation = &gardencorev1beta1.ControllerInstallation{}
	)

	mutate := func() error {
		kutil.SetMetaDataLabel(&controllerInstallation.ObjectMeta, common.SeedSpecHash, seedSpecHash)
		kutil.SetMetaDataLabel(&controllerInstallation.ObjectMeta, common.RegistrationSpecHash, registrationSpecHash)
		controllerInstallation.Spec = installationSpec
		return nil
	}

	if controllerInstallationName != "" {
		// The installation already exists, however, we do not have the latest version of the ControllerInstallation object.
		// Hence, we are running the `CreateOrUpdate` function as it first GETs the current objects and then runs the mutate()
		// function before sending the UPDATE. This way we ensure that we have applied our mutations to the latest version.
		controllerInstallation.Name = controllerInstallationName
		_, err := controllerutil.CreateOrUpdate(ctx, c, controllerInstallation, mutate)
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
	logger *logrus.Entry,
	c client.Client,
	wantedControllerRegistrationNames sets.String,
	registrationNameToInstallationName map[string]string,
) error {
	for registrationName, installationName := range registrationNameToInstallationName {
		if !wantedControllerRegistrationNames.Has(registrationName) {
			logger.Infof("Deleting unneeded ControllerInstallation %q", installationName)

			if err := c.Delete(ctx, &gardencorev1beta1.ControllerInstallation{ObjectMeta: metav1.ObjectMeta{Name: installationName}}); client.IgnoreNotFound(err) != nil {
				return err
			}
		}
	}

	return nil
}

func convertObjToMap(in interface{}) (map[string]interface{}, error) {
	var out map[string]interface{}

	data, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}

	return out, nil
}

func seedSelectorMatches(deployment *gardencorev1beta1.ControllerDeployment, seedLabels map[string]string) (bool, error) {
	selector := &metav1.LabelSelector{}
	if deployment != nil && deployment.SeedSelector != nil {
		selector = deployment.SeedSelector
	}

	seedSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return false, fmt.Errorf("label selector conversion failed: %v for seedSelector: %v", *selector, err)
	}

	return seedSelector.Matches(labels.Set(seedLabels)), nil
}

func controllerRegistrationNamesWithMatchingSeedLabelSelector(
	namesInQuestion []string,
	controllerRegistrations map[string]controllerRegistration,
	seedLabels map[string]string,
) (sets.String, error) {
	matchingNames := sets.NewString()

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

func getShoots(ctx context.Context, g gardencoreclientset.Interface, seed *gardencorev1beta1.Seed) ([]gardencorev1beta1.Shoot, error) {
	var (
		shootList  *gardencorev1beta1.ShootList
		shootList2 *gardencorev1beta1.ShootList
		err        error
	)

	if shootList, err = g.CoreV1beta1().Shoots(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{core.ShootSeedName: seed.Name}).String(),
	}); err != nil {
		return nil, err
	}

	if shootList2, err = g.CoreV1beta1().Shoots(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{core.ShootStatusSeedName: seed.Name}).String(),
	}); err != nil {
		return nil, err
	}

	shootListAsItems := gardencorev1beta1helper.ShootItems(*shootList)
	shootListAsItems2 := gardencorev1beta1helper.ShootItems(*shootList2)
	return shootListAsItems.Union(&shootListAsItems2), nil
}
