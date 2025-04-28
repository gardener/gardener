// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllerutils"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
)

// Reconciler schedules shoots to seeds.
type Reconciler struct {
	Client          client.Client
	Config          *schedulerconfigv1alpha1.ShootSchedulerConfiguration
	GardenNamespace string
	Recorder        record.EventRecorder
}

// Reconcile schedules shoots to seeds.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.Client.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if shoot.Spec.SeedName != nil {
		log.Info("Shoot already scheduled onto seed, nothing left to do", "seed", *shoot.Spec.SeedName)
		return reconcile.Result{}, nil
	}

	if shoot.DeletionTimestamp != nil {
		log.Info("Ignoring shoot because it has been marked for deletion")
		return reconcile.Result{}, nil
	}

	// If no Seed is referenced, we try to determine an adequate one.
	seed, err := r.DetermineSeed(ctx, log, shoot)
	if err != nil {
		r.reportFailedScheduling(ctx, log, shoot, err)
		return reconcile.Result{}, fmt.Errorf("failed to determine seed for shoot: %w", err)
	}

	shoot.Spec.SeedName = &seed.Name
	if err = r.Client.SubResource("binding").Update(ctx, shoot); err != nil {
		r.reportFailedScheduling(ctx, log, shoot, err)
		return reconcile.Result{}, fmt.Errorf("failed to bind shoot to seed: %w", err)
	}

	log.Info(
		"Shoot successfully scheduled to seed",
		"cloudprofile", shoot.Spec.CloudProfileName,
		"region", shoot.Spec.Region,
		"seed", seed.Name,
		"strategy", r.Config.Strategy,
	)

	r.reportEvent(shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventSchedulingSuccessful, "Scheduled to seed '%s'", seed.Name)
	return reconcile.Result{}, nil
}

func (r *Reconciler) reportFailedScheduling(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot, err error) {
	description := "Failed to schedule Shoot: " + err.Error()
	r.reportEvent(shoot, corev1.EventTypeWarning, gardencorev1beta1.ShootEventSchedulingFailed, description)

	patch := client.MergeFrom(shoot.DeepCopy())
	if shoot.Status.LastOperation == nil {
		shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{}
	}
	shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeCreate
	shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStatePending
	shoot.Status.LastOperation.LastUpdateTime = metav1.Now()
	shoot.Status.LastOperation.Description = description
	if err := r.Client.Status().Patch(ctx, shoot, patch); err != nil {
		log.Error(err, "Failed to report scheduling failure to shoot status")
	}
}

func (r *Reconciler) reportEvent(shoot *gardencorev1beta1.Shoot, eventType string, eventReason, messageFmt string, args ...any) {
	r.Recorder.Eventf(shoot, eventType, eventReason, messageFmt, args...)
}

// DetermineSeed returns an appropriate Seed cluster (or nil).
func (r *Reconciler) DetermineSeed(
	ctx context.Context,
	log logr.Logger,
	shoot *gardencorev1beta1.Shoot,
) (
	*gardencorev1beta1.Seed,
	error,
) {
	seedList := &gardencorev1beta1.SeedList{}
	if err := r.Client.List(ctx, seedList); err != nil {
		return nil, err
	}
	sl := &gardencorev1beta1.ShootList{}
	if err := r.Client.List(ctx, sl); err != nil {
		return nil, err
	}

	shootList := v1beta1helper.ConvertShootList(sl.Items)

	cloudProfile, err := gardenerutils.GetCloudProfile(ctx, r.Client, shoot)
	if err != nil {
		return nil, err
	}
	regionConfig, err := r.getRegionConfigMap(ctx, log, cloudProfile)
	if err != nil {
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
	filteredSeeds, err = filterSeedsForZonalShootControlPlanes(filteredSeeds, shoot)
	if err != nil {
		return nil, err
	}
	filteredSeeds, err = filterSeedsForAccessRestrictions(filteredSeeds, shoot)
	if err != nil {
		return nil, err
	}
	filteredSeeds, err = filterCandidates(shoot, shootList, filteredSeeds)
	if err != nil {
		return nil, err
	}
	filteredSeeds, err = applyStrategy(log, shoot, filteredSeeds, r.Config.Strategy, regionConfig)
	if err != nil {
		return nil, err
	}
	return getSeedWithLeastShootsDeployed(filteredSeeds, shootList)
}

func (r *Reconciler) getRegionConfigMap(ctx context.Context, log logr.Logger, cloudProfile *gardencorev1beta1.CloudProfile) (*corev1.ConfigMap, error) {
	regionConfigList := &corev1.ConfigMapList{}
	if err := r.Client.List(ctx, regionConfigList, client.InNamespace(r.GardenNamespace), client.MatchingLabels{v1beta1constants.SchedulingPurpose: v1beta1constants.SchedulingPurposeRegionConfig}); err != nil {
		return nil, err
	}

	var regionConfig *corev1.ConfigMap
	for _, regionConf := range regionConfigList.Items {
		profileNames := strings.Split(regionConf.Annotations[v1beta1constants.AnnotationSchedulingCloudProfiles], ",")
		for _, name := range profileNames {
			if name != cloudProfile.Name {
				continue
			}
			if regionConfig == nil {
				regionConfig = regionConf.DeepCopy()
			} else {
				log.Info("Duplicate scheduler region config found", "configMap", client.ObjectKeyFromObject(&regionConf), "cloudProfileName", cloudProfile.Name, "chosenConfigMap", client.ObjectKeyFromObject(regionConfig))
			}
			break
		}
	}

	if regionConfig == nil {
		log.Info("No region config found", "cloudProfileName", cloudProfile.Name)
	}
	return regionConfig, nil
}

func isUsableSeed(seed *gardencorev1beta1.Seed) bool {
	return seed.DeletionTimestamp == nil && seed.Spec.Settings.Scheduling.Visible && verifySeedReadiness(seed)
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
	if seedSelector == nil {
		return seedList, nil
	}
	selector, err := metav1.LabelSelectorAsSelector(&seedSelector.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("label selector conversion failed: %v for seedSelector: %w", seedSelector.LabelSelector, err)
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

	if len(matchingSeeds) == 0 {
		return nil, fmt.Errorf("none out of the %d seeds has a matching provider for %q", len(seedList), shoot.Spec.Provider.Type)
	}
	return matchingSeeds, nil
}

// filterSeedsForZonalShootControlPlanes filters seeds with at least three zones in case the shoot's failure tolerance
// type is 'zone'.
func filterSeedsForZonalShootControlPlanes(seedList []gardencorev1beta1.Seed, shoot *gardencorev1beta1.Shoot) ([]gardencorev1beta1.Seed, error) {
	if v1beta1helper.IsMultiZonalShootControlPlane(shoot) {
		var seedsWithAtLeastThreeZones []gardencorev1beta1.Seed
		for _, seed := range seedList {
			if len(seed.Spec.Provider.Zones) >= 3 {
				seedsWithAtLeastThreeZones = append(seedsWithAtLeastThreeZones, seed)
			}
		}
		if len(seedsWithAtLeastThreeZones) == 0 {
			return nil, fmt.Errorf("none of the %d seeds has at least 3 zones for hosting a shoot control plane with failure tolerance type 'zone'", len(seedList))
		}
		return seedsWithAtLeastThreeZones, nil
	}
	return seedList, nil
}

// filterSeedsForAccessRestrictions filters seeds which do not support the access restrictions configured in the shoot.
func filterSeedsForAccessRestrictions(seedList []gardencorev1beta1.Seed, shoot *gardencorev1beta1.Shoot) ([]gardencorev1beta1.Seed, error) {
	var seedsSupportingAccessRestrictions []gardencorev1beta1.Seed
	for _, seed := range seedList {
		if v1beta1helper.AccessRestrictionsAreSupported(seed.Spec.AccessRestrictions, shoot.Spec.AccessRestrictions) {
			seedsSupportingAccessRestrictions = append(seedsSupportingAccessRestrictions, seed)
		}
	}

	if len(seedsSupportingAccessRestrictions) == 0 {
		return nil, fmt.Errorf("none of the %d seeds supports the access restrictions configured in the shoot specification", len(seedList))
	}
	return seedsSupportingAccessRestrictions, nil
}

func applyStrategy(log logr.Logger, shoot *gardencorev1beta1.Shoot, seedList []gardencorev1beta1.Seed, strategy schedulerconfigv1alpha1.CandidateDeterminationStrategy, regionConfig *corev1.ConfigMap) ([]gardencorev1beta1.Seed, error) {
	var candidates []gardencorev1beta1.Seed

	switch {
	case shoot.Spec.Purpose != nil && *shoot.Spec.Purpose == gardencorev1beta1.ShootPurposeTesting:
		candidates = determineCandidatesOfSameProvider(seedList, shoot)
	case strategy == schedulerconfigv1alpha1.SameRegion:
		candidates = determineCandidatesWithSameRegionStrategy(seedList, shoot)
	case strategy == schedulerconfigv1alpha1.MinimalDistance:
		var err error
		candidates, err = determineCandidatesWithMinimalDistanceStrategy(log, shoot, seedList, regionConfig)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("failed to determine seed candidates. shoot purpose: '%s', strategy: '%s', valid strategies are: %v", *shoot.Spec.Purpose, strategy, schedulerconfigv1alpha1.Strategies)
	}

	if candidates == nil {
		var cloudProfileName string
		if shoot.Spec.CloudProfile != nil {
			cloudProfileName = shoot.Spec.CloudProfile.Name
		} else if shoot.Spec.CloudProfileName != nil {
			cloudProfileName = *shoot.Spec.CloudProfileName
		}
		return nil, fmt.Errorf("no matching seed candidate found for Configuration (Cloud Profile '%s', Region '%s', SeedDeterminationStrategy '%s')", cloudProfileName, shoot.Spec.Region, strategy)
	}
	return candidates, nil
}

func filterCandidates(shoot *gardencorev1beta1.Shoot, shootList []*gardencorev1beta1.Shoot, seedList []gardencorev1beta1.Seed) ([]gardencorev1beta1.Seed, error) {
	var (
		candidates    []gardencorev1beta1.Seed
		seedNameToErr = make(map[string]error)
		seedUsage     = v1beta1helper.CalculateSeedUsage(shootList)
	)

	for _, seed := range seedList {
		if shoot.Spec.Networking != nil {
			if disjointed, err := networksAreDisjointed(&seed, shoot); !disjointed {
				seedNameToErr[seed.Name] = err
				continue
			}
		}

		if !v1beta1helper.TaintsAreTolerated(seed.Spec.Taints, shoot.Spec.Tolerations) {
			seedNameToErr[seed.Name] = errors.New("shoot does not tolerate the seed's taints")
			continue
		}

		if allocatableShoots, ok := seed.Status.Allocatable[gardencorev1beta1.ResourceShoots]; ok && int64(seedUsage[seed.Name]) >= allocatableShoots.Value() {
			seedNameToErr[seed.Name] = errors.New("seed does not have available capacity for shoots")
			continue
		}

		candidates = append(candidates, seed)
	}

	if candidates == nil {
		return nil, fmt.Errorf("0/%d seed cluster candidate(s) are eligible for scheduling: %v", len(seedList), errorMapToString(seedNameToErr))
	}
	return candidates, nil
}

// getSeedWithLeastShootsDeployed finds the best candidate (i.e. the one managing the smallest number of shoots right now).
func getSeedWithLeastShootsDeployed(seedList []gardencorev1beta1.Seed, shootList []*gardencorev1beta1.Shoot) (*gardencorev1beta1.Seed, error) {
	var (
		bestCandidate gardencorev1beta1.Seed
		min           *int
		seedUsage     = v1beta1helper.CalculateSeedUsage(shootList)
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

func determineCandidatesWithMinimalDistanceStrategy(log logr.Logger, shoot *gardencorev1beta1.Shoot, seedList []gardencorev1beta1.Seed, regionConfig *corev1.ConfigMap) ([]gardencorev1beta1.Seed, error) {
	candidates, err := regionConfigMinimalDistance(log, seedList, shoot, regionConfig)
	if err != nil {
		return nil, err
	}

	// Fall back to Levenshtein minimal distance in case we didn't find any candidates.
	if len(candidates) == 0 {
		log.Info("No candidates found with minimal distance of region config. Falling back to Levenshtein minimal distance")
		candidates = levenshteinMinimalDistance(seedList, shoot)
	}
	return candidates, nil
}

func regionConfigMinimalDistance(log logr.Logger, seeds []gardencorev1beta1.Seed, shoot *gardencorev1beta1.Shoot, regionConfig *corev1.ConfigMap) ([]gardencorev1beta1.Seed, error) {
	var candidates []gardencorev1beta1.Seed

	if regionConfig == nil || regionConfig.Data[shoot.Spec.Region] == "" {
		log.Info("Region ConfigMap not provided or Shoot region not available", "region", shoot.Spec.Region)
		return candidates, nil
	}

	regionConfigData := make(map[string]int)
	if err := yaml.Unmarshal([]byte(regionConfig.Data[shoot.Spec.Region]), &regionConfigData); err != nil {
		return nil, fmt.Errorf("failed to determine seed candidates. Wrong format in region ConfigMap %s/%s, Region %q: %w", regionConfig.Namespace, regionConfig.Name, shoot.Spec.Region, err)
	}

	// If not configured otherwise, assume that a region has the smallest possible distance to itself.
	if _, ok := regionConfigData[shoot.Spec.Region]; !ok {
		regionConfigData[shoot.Spec.Region] = 0
	}

	minDistance := math.MaxInt32
	for _, seed := range seeds {
		dist, ok := regionConfigData[seed.Spec.Provider.Region]
		if !ok {
			log.Info("Seed region not available in scheduler region ConfigMap for shoot region", "seedName", seed.Name, "shootRegion", shoot.Spec.Region, "seedRegion", seed.Spec.Provider.Region)
			continue
		}

		if dist == minDistance {
			candidates = append(candidates, seed)
			continue
		}

		if dist < minDistance {
			minDistance = dist
			candidates = []gardencorev1beta1.Seed{seed}
		}
	}

	return candidates, nil
}

func levenshteinMinimalDistance(seeds []gardencorev1beta1.Seed, shoot *gardencorev1beta1.Shoot) []gardencorev1beta1.Seed {
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

		if dist == minDistance {
			candidates = append(candidates, seed)
			continue
		}

		if dist < minDistance {
			minDistance = dist
			candidates = []gardencorev1beta1.Seed{seed}
		}
	}
	return candidates
}

func networksAreDisjointed(seed *gardencorev1beta1.Seed, shoot *gardencorev1beta1.Shoot) (bool, error) {
	var (
		shootPodsNetwork     = shoot.Spec.Networking.Pods
		shootServicesNetwork = shoot.Spec.Networking.Services

		errorMessages []string
		workerless    = v1beta1helper.IsWorkerless(shoot)
		haVPN         = v1beta1helper.IsHAVPNEnabled(shoot)
	)

	if seed.Spec.Networks.ShootDefaults != nil {
		if shootPodsNetwork == nil && !workerless {
			if cidrvalidation.NewCIDR(*seed.Spec.Networks.ShootDefaults.Pods, field.NewPath("spec", "networks", "shootDefaults", "pods")).IsIPv6() &&
				slices.Contains(shoot.Spec.Networking.IPFamilies, gardencorev1beta1.IPFamilyIPv6) ||
				cidrvalidation.NewCIDR(*seed.Spec.Networks.ShootDefaults.Pods, field.NewPath("spec", "networks", "shootDefaults", "pods")).IsIPv4() &&
					slices.Contains(shoot.Spec.Networking.IPFamilies, gardencorev1beta1.IPFamilyIPv4) {
				shootPodsNetwork = seed.Spec.Networks.ShootDefaults.Pods
			}
		}
		if shootServicesNetwork == nil {
			if cidrvalidation.NewCIDR(*seed.Spec.Networks.ShootDefaults.Services, field.NewPath("spec", "networks", "shootDefaults", "services")).IsIPv6() &&
				slices.Contains(shoot.Spec.Networking.IPFamilies, gardencorev1beta1.IPFamilyIPv6) ||
				cidrvalidation.NewCIDR(*seed.Spec.Networks.ShootDefaults.Services, field.NewPath("spec", "networks", "shootDefaults", "services")).IsIPv4() &&
					slices.Contains(shoot.Spec.Networking.IPFamilies, gardencorev1beta1.IPFamilyIPv4) {
				shootServicesNetwork = seed.Spec.Networks.ShootDefaults.Services
			}
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
		!haVPN,
	) {
		errorMessages = append(errorMessages, e.ErrorBody())
	}

	if shoot.Status.Networking != nil {
		for _, e := range cidrvalidation.ValidateMultiNetworkDisjointedness(
			field.NewPath(""),
			shoot.Status.Networking.Nodes,
			shoot.Status.Networking.Pods,
			shoot.Status.Networking.Services,
			seed.Spec.Networks.Nodes,
			seed.Spec.Networks.Pods,
			seed.Spec.Networks.Services,
			workerless,
			!haVPN,
		) {
			errorMessages = append(errorMessages, e.ErrorBody())
		}
	}

	return len(errorMessages) == 0, fmt.Errorf("invalid networks: %s", errorMessages)
}

func errorMapToString(seedNameToErr map[string]error) string {
	sortedSeeds := maps.Keys(seedNameToErr)
	slices.Sort(sortedSeeds)

	res := "{"
	for _, seed := range sortedSeeds {
		res += fmt.Sprintf("%s => %s, ", seed, seedNameToErr[seed].Error())
	}
	res = strings.TrimSuffix(res, ", ") + "}"
	return res
}

func verifySeedReadiness(seed *gardencorev1beta1.Seed) bool {
	if seed.Status.LastOperation == nil {
		return false
	}

	if cond := v1beta1helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedGardenletReady); cond == nil || cond.Status != gardencorev1beta1.ConditionTrue {
		return false
	}

	if seed.Spec.Backup != nil {
		if cond := v1beta1helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedBackupBucketsReady); cond == nil || cond.Status != gardencorev1beta1.ConditionTrue {
			return false
		}
	}

	return true
}
