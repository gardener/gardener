// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seedmanager

import (
	"errors"
	"fmt"
	"io"
	"strings"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	"github.com/gardener/gardener/pkg/apis/garden"
	gardenhelper "github.com/gardener/gardener/pkg/apis/garden/helper"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/plugin/pkg/shoot/seedmanager/validation"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/plugin/pkg/shoot/seedmanager/apis/seedmanager"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ShootSeedManager"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		// load the configuration provided (if any)
		configuration, err := LoadConfiguration(config)
		if err != nil {
			return nil, err
		}

		// validate the configuration
		if err := validation.ValidateConfiguration(configuration); err != nil {
			return nil, err
		}

		return New(configuration)
	})
}

// SeedManager contains listers and and admission handler.
type SeedManager struct {
	*admission.Handler
	seedLister  gardenlisters.SeedLister
	shootLister gardenlisters.ShootLister
	readyFunc   admission.ReadyFunc
	strategy    seedmanager.CandidateDeterminationStrategy
}

var (
	_ = admissioninitializer.WantsInternalGardenInformerFactory(&SeedManager{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new SeedManager admission plugin.
func New(configuration *seedmanager.Configuration) (*SeedManager, error) {
	return &SeedManager{
		Handler:  admission.NewHandler(admission.Create, admission.Update),
		strategy: configuration.Strategy,
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (s *SeedManager) AssignReadyFunc(f admission.ReadyFunc) {
	s.readyFunc = f
	s.SetReadyFunc(f)
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (s *SeedManager) SetInternalGardenInformerFactory(f gardeninformers.SharedInformerFactory) {
	seedInformer := f.Garden().InternalVersion().Seeds()
	s.seedLister = seedInformer.Lister()

	shootInformer := f.Garden().InternalVersion().Shoots()
	s.shootLister = shootInformer.Lister()

	readyFuncs = append(readyFuncs, seedInformer.Informer().HasSynced, shootInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (s *SeedManager) ValidateInitialization() error {
	if s.seedLister == nil {
		return errors.New("missing seed lister")
	}
	if s.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	return nil
}

// Admit tries to find an adequate Seed cluster for the given cloud provider profile and region,
// and writes the name into the Shoot specification. It also ensures that protected Seeds are
// only usable by Shoots in the garden namespace.
func (s *SeedManager) Admit(a admission.Attributes, o admission.ObjectInterfaces) error {
	// Wait until the caches have been synced
	if s.readyFunc == nil {
		s.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
	if !s.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	// Ignore all kinds other than Shoot
	if a.GetKind().GroupKind() != garden.Kind("Shoot") {
		return nil
	}
	shoot, ok := a.GetObject().(*garden.Shoot)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Shoot object")
	}

	// If the Shoot manifest already specifies a desired Seed cluster, then we check whether it is protected or not.
	// In case it is protected then we only allow Shoot resources to reference it which are part of the Garden namespace.
	// Also, we don't allow shoot to be created on the seed which is already marked to be deleted.
	if shoot.Spec.Cloud.Seed != nil {
		seed, err := s.seedLister.Get(*shoot.Spec.Cloud.Seed)
		if err != nil {
			return admission.NewForbidden(a, err)
		}

		if shoot.Namespace != common.GardenNamespace && seed.Spec.Protected != nil && *seed.Spec.Protected {
			return admission.NewForbidden(a, errors.New("forbidden to use a protected seed"))
		}

		if a.GetOperation() == admission.Create && seed.DeletionTimestamp != nil {
			return admission.NewForbidden(a, errors.New("forbidden to use a seed marked to be deleted"))
		}

		if hasDisjointedNetworks, allErrs := validateDisjointedNetworks(seed, shoot); !hasDisjointedNetworks {
			return admission.NewForbidden(a, allErrs.ToAggregate())
		}

		return nil
	}

	// If no Seed is referenced, we try to determine an adequate one.
	seed, err := determineSeed(shoot, s.seedLister, s.shootLister, s.strategy)
	if err != nil {
		return admission.NewForbidden(a, err)
	}

	shoot.Spec.Cloud.Seed = &seed.Name
	return nil
}

// determineSeed returns an appropriate Seed cluster (or nil).
func determineSeed(shoot *garden.Shoot, seedLister gardenlisters.SeedLister, shootLister gardenlisters.ShootLister, strategy seedmanager.CandidateDeterminationStrategy) (*garden.Seed, error) {
	seedList, err := seedLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	shootList, err := shootLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	// Map seeds to number of managed shoots.
	var (
		seedUsage  = generateSeedUsageMap(shootList)
		candidates []*garden.Seed
	)

	switch strategy {
	case seedmanager.SameRegion:
		candidates = determineCandidatesWithSameRegionStrategy(seedList, shoot, candidates)
	case seedmanager.MinimalDistance:
		candidates = determineCandidatesWithMinimalDistanceStrategy(seedList, shoot, candidates)
	default:
		return nil, fmt.Errorf("unknown seed determination strategy configured in seed admission plugin. Strategy: '%s' does not exist. Valid strategies are: %v", strategy, seedmanager.Strategies)
	}

	if candidates == nil {
		return nil, errors.New("no adequate seed cluster found for this cloud profile and region")
	}

	old := candidates
	candidates = nil

	for _, seed := range old {
		if hasDisjointedNetworks, _ := validateDisjointedNetworks(seed, shoot); hasDisjointedNetworks {
			candidates = append(candidates, seed)
		}
	}

	if candidates == nil {
		return nil, errors.New("no adequate seed cluster found with disjoint network")
	}

	var (
		bestCandidate *garden.Seed
		min           *int
	)

	// Find the best candidate (i.e. the one managing the smallest number of shoots right now).
	for _, seed := range candidates {
		if numberOfManagedShoots := seedUsage[seed.Name]; min == nil || numberOfManagedShoots < *min {
			bestCandidate = seed
			min = &numberOfManagedShoots
		}
	}

	return bestCandidate, nil
}

func determineCandidatesWithSameRegionStrategy(seedList []*garden.Seed, shoot *garden.Shoot, candidates []*garden.Seed) []*garden.Seed {
	// Determine all candidate seed clusters matching the shoot's cloud and region.
	for _, seed := range seedList {
		if seed.DeletionTimestamp == nil && seed.Spec.Cloud.Profile == shoot.Spec.Cloud.Profile && seed.Spec.Cloud.Region == shoot.Spec.Cloud.Region && seed.Spec.Visible != nil && *seed.Spec.Visible && verifySeedAvailability(seed) {
			candidates = append(candidates, seed)
		}
	}
	return candidates
}

func determineCandidatesWithMinimalDistanceStrategy(seeds []*garden.Seed, shoot *garden.Shoot, candidates []*garden.Seed) []*garden.Seed {
	if candidates = determineCandidatesWithSameRegionStrategy(seeds, shoot, candidates); candidates != nil {
		return candidates
	}

	var (
		currentMaxMatchingCharacters int
		shootRegion                  = shoot.Spec.Cloud.Region
	)

	// Determine all candidate seed clusters with matching cloud provider but different region that are lexicographically closest to the shoot
	for _, seed := range seeds {
		if seed.DeletionTimestamp == nil && seed.Spec.Cloud.Profile == shoot.Spec.Cloud.Profile && seed.Spec.Visible != nil && *seed.Spec.Visible && verifySeedAvailability(seed) {
			seedRegion := seed.Spec.Cloud.Region

			for currentMaxMatchingCharacters < len(shootRegion) {
				if strings.HasPrefix(seedRegion, shootRegion[:currentMaxMatchingCharacters+1]) {
					candidates = []*garden.Seed{}
					currentMaxMatchingCharacters++
					continue
				} else if strings.HasPrefix(seedRegion, shootRegion[:currentMaxMatchingCharacters]) {
					candidates = append(candidates, seed)
				}
				break
			}
		}
	}
	return candidates
}

func generateSeedUsageMap(shootList []*garden.Shoot) map[string]int {
	m := map[string]int{}

	for _, shoot := range shootList {
		if seed := shoot.Spec.Cloud.Seed; seed != nil {
			m[*seed]++
		}
	}

	return m
}

func verifySeedAvailability(seed *garden.Seed) bool {
	if cond := gardencorehelper.GetCondition(seed.Status.Conditions, garden.SeedAvailable); cond != nil {
		return cond.Status == gardencore.ConditionTrue
	}
	return false
}

func validateDisjointedNetworks(seed *garden.Seed, shoot *garden.Shoot) (bool, field.ErrorList) {
	// error cannot occur due to our static validation
	k8sNetworks, _ := gardenhelper.GetK8SNetworks(shoot)
	allErrs := admissionutils.ValidateNetworkDisjointedness(seed.Spec.Networks, k8sNetworks, field.NewPath(""))
	return len(allErrs) == 0, allErrs
}
