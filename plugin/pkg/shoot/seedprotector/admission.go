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

package seedprotector

import (
	"errors"
	"io"

	"github.com/gardener/gardener/pkg/apis/garden"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ShootSeedProtector"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// Protector contains listers and admisson handler.
type Protector struct {
	*admission.Handler
	seedLister gardenlisters.SeedLister
}

var _ = admissioninitializer.WantsInternalGardenInformerFactory(&Protector{})

// New creates an new Protector admission plugin.
func New() (*Protector, error) {
	return &Protector{
		Handler: admission.NewHandler(admission.Create),
	}, nil
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (h *Protector) SetInternalGardenInformerFactory(f gardeninformers.SharedInformerFactory) {
	h.seedLister = f.Garden().InternalVersion().Seeds().Lister()
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (h *Protector) ValidateInitialization() error {
	if h.seedLister == nil {
		return errors.New("missing seed lister")
	}
	return nil
}

// Admit ensures that a Shoot can't use a protected Seed cluster.
// Protected shoots not allowed to use for regular Shoot cluster control planes.
func (h *Protector) Admit(a admission.Attributes) error {
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

	// If the Shoot does not specify a Seed, then the admisson controller will do nothing.
	if shoot.Spec.Cloud.Seed == nil {
		return nil
	}

	// Get the Seed manifest, which is refrenced by the Shoot.
	seed, err := h.seedLister.Get(*shoot.Spec.Cloud.Seed)
	if err != nil {
		return admission.NewForbidden(a, err)
	}

	// Protected Seeds can be only used by shoots in garden Namespace
	if shoot.Namespace != common.GardenNamespace && seed.Spec.Protected != nil && *seed.Spec.Protected {
		return admission.NewForbidden(a, errors.New("forbidden to use a protected seed"))
	}
	return nil
}
