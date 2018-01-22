// Copyright 2018 The Gardener Authors.
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

package seedfinder

import (
	"errors"
	"io"

	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/helper"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register("ShootSeedFinder", func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// Finder contains listers and and admission handler.
type Finder struct {
	*admission.Handler
	seedLister gardenlisters.SeedLister
}

var _ = admissioninitializer.WantsInternalGardenInformerFactory(&Finder{})

// New creates a new Finder admission plugin.
func New() (*Finder, error) {
	return &Finder{
		Handler: admission.NewHandler(admission.Create),
	}, nil
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (h *Finder) SetInternalGardenInformerFactory(f gardeninformers.SharedInformerFactory) {
	h.seedLister = f.Garden().InternalVersion().Seeds().Lister()
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (h *Finder) ValidateInitialization() error {
	if h.seedLister == nil {
		return errors.New("missing seed lister")
	}
	return nil
}

// Admit ensures that the object in-flight is of kind Shoot.
// In addition it tries to find an adequate Seed cluster for the given cloud provider profile and region,
// and writes the name into the Shoot specification.
func (h *Finder) Admit(a admission.Attributes) error {
	// Wait until the caches have been synced
	if !h.WaitForReady() {
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

	// If the Shoot manifest already specifies a desired Seed cluster, then we do nothing.
	if shoot.Spec.Cloud.Seed != nil {
		return nil
	}

	seed, err := determineSeed(shoot, h.seedLister)
	if err != nil {
		return admission.NewForbidden(a, err)
	}

	shoot.Spec.Cloud.Seed = &seed.Name
	return nil
}

// determineSeed returns an approriate Seed cluster (or nil).
func determineSeed(shoot *garden.Shoot, lister gardenlisters.SeedLister) (*garden.Seed, error) {
	list, err := lister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, seed := range list {
		// We return the first matching seed cluster.
		if seed.Spec.Cloud.Profile == shoot.Spec.Cloud.Profile && seed.Spec.Cloud.Region == shoot.Spec.Cloud.Region && verifySeedAvailability(seed) {
			return seed, nil
		}
	}

	return nil, errors.New("failed to determine an adequate Seed cluster for this cloud profile and region")
}

func verifySeedAvailability(seed *garden.Seed) bool {
	if cond := helper.GetCondition(seed.Status.Conditions, garden.SeedAvailable); cond != nil {
		return cond.Status == corev1.ConditionTrue
	}
	return false
}
