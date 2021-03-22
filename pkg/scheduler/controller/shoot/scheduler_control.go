// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"context"
	"fmt"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	"github.com/gardener/gardener/pkg/scheduler/controller/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// MsgUnschedulable is the Message for the Event on a Shoot that the Scheduler creates in case it cannot schedule the Shoot to any Seed
const MsgUnschedulable = "Failed to schedule shoot"

func (c *SchedulerController) shootAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	newShoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		logger.Logger.Errorf("Couldn't convert object into `core.gardener.cloud/v1beta1.Shoot`: %+v: %v", obj, err)
		return
	}

	// If the Shoot manifest already specifies a desired Seed cluster, we ignore it.
	if newShoot.Spec.SeedName != nil {
		return
	}

	if newShoot.DeletionTimestamp != nil {
		logger.Logger.Infof("Ignoring shoot '%s' because it has been marked for deletion", newShoot.Name)
		c.shootQueue.Forget(key)
		return
	}

	c.shootQueue.Add(key)
}

func (c *SchedulerController) shootUpdate(_, newObj interface{}) {
	c.shootAdd(newObj)
}

// NewReconciler creates a new instance of a reconciler which schedules Shoots.
func NewReconciler(
	l logrus.FieldLogger,
	config *config.SchedulerConfiguration,
	gardenClient kubernetes.Interface,
	recorder record.EventRecorder,
) reconcile.Reconciler {
	return &reconciler{
		logger:       l,
		config:       config,
		gardenClient: gardenClient,
		recorder:     recorder,
	}
}

type reconciler struct {
	logger       logrus.FieldLogger
	config       *config.SchedulerConfiguration
	gardenClient kubernetes.Interface
	recorder     record.EventRecorder
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.Client().Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Infof("Object %q is gone, stop reconciling: %v", request.Name, err)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("Unable to retrieve object %q from store: %v", request.Name, err)
		return reconcile.Result{}, err
	}

	schedulerLogger := logger.NewFieldLogger(logger.Logger, "scheduler", "shoot").WithField("shoot", shoot.Name)

	// If no Seed is referenced, we try to determine an adequate one.
	seed, err := determineSeed(ctx, r.gardenClient.Cache(), shoot, r.config.Schedulers.Shoot.Strategy)
	if err != nil {
		r.reportFailedScheduling(shoot, err)
		return reconcile.Result{}, err
	}

	updateShoot := func(ctx context.Context, shootToUpdate *gardencorev1beta1.Shoot) error {
		// need retry logic, because the controller-manager is acting on it at the same time: setting Status to Pending until scheduled
		_, err = kutil.TryUpdateShoot(ctx, r.gardenClient.GardenCore(), retry.DefaultBackoff, shootToUpdate.ObjectMeta, func(shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
			if shoot.Spec.SeedName != nil {
				alreadyScheduledErr := common.NewAlreadyScheduledError(fmt.Sprintf("shoot has already a seed assigned when trying to schedule the shoot to %s", *shootToUpdate.Spec.SeedName))
				return nil, &alreadyScheduledErr
			}
			shoot.Spec.SeedName = shootToUpdate.Spec.SeedName
			return shoot, nil
		})
		return err
	}

	if err := UpdateShootToBeScheduledOntoSeed(ctx, shoot, seed, updateShoot); err != nil {
		// there was an external change while trying to schedule the shoot. The shoot is already scheduled. Fine, do not raise an error.
		if _, ok := err.(*common.AlreadyScheduledError); ok {
			return reconcile.Result{}, nil
		}
		r.reportFailedScheduling(shoot, err)
		return reconcile.Result{}, err
	}

	schedulerLogger.Infof("Shoot '%s' (Cloud Profile '%s', Region '%s') successfully scheduled to seed '%s' using SeedDeterminationStrategy '%s'", shoot.Name, shoot.Spec.CloudProfileName, shoot.Spec.Region, seed.Name, r.config.Schedulers.Shoot.Strategy)
	r.reportEvent(shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventSchedulingSuccessful, "Scheduled to seed '%s'", seed.Name)
	return reconcile.Result{}, nil
}

func (r *reconciler) reportFailedScheduling(shoot *gardencorev1beta1.Shoot, err error) {
	r.reportEvent(shoot, corev1.EventTypeWarning, gardencorev1beta1.ShootEventSchedulingFailed, MsgUnschedulable+" '%s' : %+v", shoot.Name, err)
}

func (r *reconciler) reportEvent(project *gardencorev1beta1.Shoot, eventType string, eventReason, messageFmt string, args ...interface{}) {
	r.recorder.Eventf(project, eventType, eventReason, messageFmt, args...)
}

// determineSeed returns an appropriate Seed cluster (or nil).
func determineSeed(
	ctx context.Context,
	reader client.Reader,
	shoot *gardencorev1beta1.Shoot,
	strategy config.CandidateDeterminationStrategy,
) (
	*gardencorev1beta1.Seed,
	error,
) {
	seedList := &gardencorev1beta1.SeedList{}
	if err := reader.List(ctx, seedList); err != nil {
		return nil, err
	}
	shootList := &gardencorev1beta1.ShootList{}
	if err := reader.List(ctx, shootList); err != nil {
		return nil, err
	}
	cloudProfile := &gardencorev1beta1.CloudProfile{}
	if err := reader.Get(ctx, kutil.Key(shoot.Spec.CloudProfileName), cloudProfile); err != nil {
		return nil, err
	}
	filteredSeeds, err := filterUsableSeeds(seedList.Items)
	if err != nil {
		return nil, err
	}
	filteredSeeds, err = filterSeedsMatchingLabelSelector(filteredSeeds, cloudProfile.Spec.SeedSelector, "CloudProfile")
	if err != nil {
		return nil, err
	}
	filteredSeeds, err = filterSeedsMatchingLabelSelector(filteredSeeds, shoot.Spec.SeedSelector, "Shoot")
	if err != nil {
		return nil, err
	}
	filteredSeeds, err = filterSeedsMatchingProviders(cloudProfile, shoot, filteredSeeds)
	if err != nil {
		return nil, err
	}
	filteredSeeds, err = filterCandidates(shoot, shootList.Items, filteredSeeds)
	if err != nil {
		return nil, err
	}
	filteredSeeds, err = applyStrategy(shoot, filteredSeeds, strategy)
	if err != nil {
		return nil, err
	}
	return getSeedWithLeastShootsDeployed(filteredSeeds, shootList.Items)
}

func isUsableSeed(seed *gardencorev1beta1.Seed) bool {
	return seed.DeletionTimestamp == nil && seed.Spec.Settings.Scheduling.Visible && common.VerifySeedReadiness(seed)
}

func filterUsableSeeds(seedList []gardencorev1beta1.Seed) ([]gardencorev1beta1.Seed, error) {
	var matchingSeeds []gardencorev1beta1.Seed

	for _, seed := range seedList {
		if isUsableSeed(&seed) {
			matchingSeeds = append(matchingSeeds, seed)
		}
	}

	if len(matchingSeeds) == 0 {
		return nil, fmt.Errorf("none of the %d seeds is valid for scheduling (not deleting, visible and ready)", len(seedList))
	}
	return matchingSeeds, nil
}

func filterSeedsMatchingLabelSelector(seedList []gardencorev1beta1.Seed, seedSelector *gardencorev1beta1.SeedSelector, kind string) ([]gardencorev1beta1.Seed, error) {
	if seedSelector == nil || seedSelector.LabelSelector == nil {
		return seedList, nil
	}
	selector, err := metav1.LabelSelectorAsSelector(seedSelector.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("label selector conversion failed: %v for seedSelector: %v", *seedSelector.LabelSelector, err)
	}

	var matchingSeeds []gardencorev1beta1.Seed
	for _, seed := range seedList {
		if selector.Matches(labels.Set(seed.Labels)) {
			matchingSeeds = append(matchingSeeds, seed)
		}
	}

	if len(matchingSeeds) == 0 {
		return nil, fmt.Errorf("none out of the %d seeds has the matching labels required by seed selector of '%s' (selector: '%s')", len(seedList), kind, selector.String())
	}
	return matchingSeeds, nil
}

func filterSeedsMatchingProviders(cloudProfile *gardencorev1beta1.CloudProfile, shoot *gardencorev1beta1.Shoot, seedList []gardencorev1beta1.Seed) ([]gardencorev1beta1.Seed, error) {
	var possibleProviders []string
	if cloudProfile.Spec.SeedSelector != nil {
		possibleProviders = cloudProfile.Spec.SeedSelector.ProviderTypes
	}

	var matchingSeeds []gardencorev1beta1.Seed
	for _, seed := range seedList {
		if matchProvider(seed.Spec.Provider.Type, shoot.Spec.Provider.Type, possibleProviders) {
			matchingSeeds = append(matchingSeeds, seed)
		}
	}
	return matchingSeeds, nil
}

func applyStrategy(shoot *gardencorev1beta1.Shoot, seedList []gardencorev1beta1.Seed, strategy config.CandidateDeterminationStrategy) ([]gardencorev1beta1.Seed, error) {
	var candidates []gardencorev1beta1.Seed

	switch {
	case shoot.Spec.Purpose != nil && *shoot.Spec.Purpose == gardencorev1beta1.ShootPurposeTesting:
		candidates = determineCandidatesOfSameProvider(seedList, shoot)
	case strategy == config.SameRegion:
		candidates = determineCandidatesWithSameRegionStrategy(seedList, shoot)
	case strategy == config.MinimalDistance:
		candidates = determineCandidatesWithMinimalDistanceStrategy(seedList, shoot)
	default:
		return nil, fmt.Errorf("failed to determine seed candidates. shoot purpose: '%s', strategy: '%s', valid strategies are: %v", *shoot.Spec.Purpose, strategy, config.Strategies)
	}

	if candidates == nil {
		return nil, fmt.Errorf("no matching seed candidate found for Configuration (Cloud Profile '%s', Region '%s', SeedDeterminationStrategy '%s')", shoot.Spec.CloudProfileName, shoot.Spec.Region, strategy)
	}
	return candidates, nil
}

func filterCandidates(shoot *gardencorev1beta1.Shoot, shootList []gardencorev1beta1.Shoot, seedList []gardencorev1beta1.Seed) ([]gardencorev1beta1.Seed, error) {
	var (
		candidates      []gardencorev1beta1.Seed
		candidateErrors = make(map[string]error)
		seedUsage       = generateSeedUsageMap(shootList)
	)

	for _, seed := range seedList {
		if disjointed, err := networksAreDisjointed(&seed, shoot); !disjointed {
			candidateErrors[seed.Name] = err
			continue
		}

		if ignoreSeedDueToDNSConfiguration(&seed, shoot) {
			candidateErrors[seed.Name] = fmt.Errorf("seed does not support DNS")
			continue
		}

		if !gardencorev1beta1helper.TaintsAreTolerated(seed.Spec.Taints, shoot.Spec.Tolerations) {
			candidateErrors[seed.Name] = fmt.Errorf("shoot does not tolerate the seed's taints")
			continue
		}

		if allocatableShoots, ok := seed.Status.Allocatable[gardencorev1beta1.ResourceShoots]; ok && int64(seedUsage[seed.Name]) >= allocatableShoots.Value() {
			candidateErrors[seed.Name] = fmt.Errorf("seed does not have available capacity for shoots")
			continue
		}

		candidates = append(candidates, seed)
	}

	if candidates == nil {
		return nil, fmt.Errorf("0/%d seed cluster candidate(s) are eligible for scheduling: %v", len(seedList), errorMapToString(candidateErrors))
	}
	return candidates, nil
}

// getSeedWithLeastShootsDeployed finds the best candidate (i.e. the one managing the smallest number of shoots right now).
func getSeedWithLeastShootsDeployed(seedList []gardencorev1beta1.Seed, shootList []gardencorev1beta1.Shoot) (*gardencorev1beta1.Seed, error) {
	var (
		bestCandidate gardencorev1beta1.Seed
		min           *int
		seedUsage     = generateSeedUsageMap(shootList)
	)

	for _, seed := range seedList {
		if numberOfManagedShoots := seedUsage[seed.Name]; min == nil || numberOfManagedShoots < *min {
			bestCandidate = seed
			min = &numberOfManagedShoots
		}
	}

	return &bestCandidate, nil
}

func matchProvider(seedProviderType, shootProviderType string, enabledProviderTypes []string) bool {
	if len(enabledProviderTypes) == 0 {
		return seedProviderType == shootProviderType
	}
	for _, p := range enabledProviderTypes {
		if p == "*" || p == seedProviderType {
			return true
		}
	}
	return false
}

func determineCandidatesOfSameProvider(seedList []gardencorev1beta1.Seed, shoot *gardencorev1beta1.Shoot) []gardencorev1beta1.Seed {
	var candidates []gardencorev1beta1.Seed
	// Determine all candidate seed clusters matching the shoot's provider and region.
	for _, seed := range seedList {
		if seed.Spec.Provider.Type == shoot.Spec.Provider.Type {
			candidates = append(candidates, seed)
		}
	}
	return candidates
}

// determineCandidatesWithSameRegionStrategy get all seed clusters matching the shoot's provider and region.
func determineCandidatesWithSameRegionStrategy(seedList []gardencorev1beta1.Seed, shoot *gardencorev1beta1.Shoot) []gardencorev1beta1.Seed {
	var candidates []gardencorev1beta1.Seed
	for _, seed := range seedList {
		if seed.Spec.Provider.Type == shoot.Spec.Provider.Type && seed.Spec.Provider.Region == shoot.Spec.Region {
			candidates = append(candidates, seed)
		}
	}
	return candidates
}

func determineCandidatesWithMinimalDistanceStrategy(seeds []gardencorev1beta1.Seed, shoot *gardencorev1beta1.Shoot) []gardencorev1beta1.Seed {
	var (
		minDistance   = 1000
		shootRegion   = shoot.Spec.Region
		shootProvider = shoot.Spec.Provider.Type
		candidates    []gardencorev1beta1.Seed
	)

	for _, seed := range seeds {
		seedRegion := seed.Spec.Provider.Region
		dist := distance(seedRegion, shootRegion)

		if shootProvider != seed.Spec.Provider.Type {
			dist = dist + 2
		}
		// append
		if dist == minDistance {
			candidates = append(candidates, seed)
			continue
		}
		// replace
		if dist < minDistance {
			minDistance = dist
			candidates = []gardencorev1beta1.Seed{seed}
		}
	}
	return candidates
}

func generateSeedUsageMap(shootList []gardencorev1beta1.Shoot) map[string]int {
	m := map[string]int{}

	for _, shoot := range shootList {
		if seed := shoot.Spec.SeedName; seed != nil {
			m[*seed]++
		}
	}

	return m
}

func networksAreDisjointed(seed *gardencorev1beta1.Seed, shoot *gardencorev1beta1.Shoot) (bool, error) {
	var (
		shootPodsNetwork     = shoot.Spec.Networking.Pods
		shootServicesNetwork = shoot.Spec.Networking.Services

		errorMessages []string
	)

	if seed.Spec.Networks.ShootDefaults != nil {
		if shootPodsNetwork == nil {
			shootPodsNetwork = seed.Spec.Networks.ShootDefaults.Pods
		}
		if shootServicesNetwork == nil {
			shootServicesNetwork = seed.Spec.Networks.ShootDefaults.Services
		}
	}

	for _, e := range cidrvalidation.ValidateNetworkDisjointedness(
		field.NewPath(""),
		shoot.Spec.Networking.Nodes,
		shootPodsNetwork,
		shootServicesNetwork,
		seed.Spec.Networks.Nodes,
		seed.Spec.Networks.Pods,
		seed.Spec.Networks.Services,
	) {
		errorMessages = append(errorMessages, e.ErrorBody())
	}

	return len(errorMessages) == 0, fmt.Errorf("invalid networks: %s", errorMessages)
}

// ignore seed if it disables DNS and shoot has DNS but not unmanaged
func ignoreSeedDueToDNSConfiguration(seed *gardencorev1beta1.Seed, shoot *gardencorev1beta1.Shoot) bool {
	if seed.Spec.Settings.ShootDNS.Enabled {
		return false
	}
	if shoot.Spec.DNS == nil {
		return false
	}
	return !gardencorev1beta1helper.ShootUsesUnmanagedDNS(shoot)
}

type executeSchedulingRequest = func(context.Context, *gardencorev1beta1.Shoot) error

// UpdateShootToBeScheduledOntoSeed sets the seed name where the shoot should be scheduled on. Then it executes the actual update call to the API server. The call is capsuled to allow for easier testing.
func UpdateShootToBeScheduledOntoSeed(ctx context.Context, shoot *gardencorev1beta1.Shoot, seed *gardencorev1beta1.Seed, executeSchedulingRequest executeSchedulingRequest) error {
	shoot.Spec.SeedName = &seed.Name
	return executeSchedulingRequest(ctx, shoot)
}

func errorMapToString(errs map[string]error) string {
	res := "{"
	for k, v := range errs {
		res += fmt.Sprintf("%s => %s, ", k, v.Error())
	}
	res = strings.TrimSuffix(res, ", ") + "}"
	return res
}
