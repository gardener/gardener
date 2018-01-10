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

package quotavalidator

import (
	"errors"
	"fmt"
	"io"

	"github.com/gardener/gardener/pkg/apis/garden"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	informers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	listers "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register("ShootQuotaValidator", func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// RejectShootIfQuotaExceeded contains listers and and admission handler.
type RejectShootIfQuotaExceeded struct {
	*admission.Handler
	shootLister        listers.ShootLister
	cloudProfileLister listers.CloudProfileLister
}

var _ = admissioninitializer.WantsInternalGardenInformerFactory(&RejectShootIfQuotaExceeded{})

// New creates a new RejectShootIfQuotaExceeded admission plugin.
func New() (*RejectShootIfQuotaExceeded, error) {
	return &RejectShootIfQuotaExceeded{
		Handler: admission.NewHandler(admission.Create),
	}, nil
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (h *RejectShootIfQuotaExceeded) SetInternalGardenInformerFactory(f informers.SharedInformerFactory) {
	h.shootLister = f.Garden().InternalVersion().Shoots().Lister()
	h.cloudProfileLister = f.Garden().InternalVersion().CloudProfiles().Lister()
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (h *RejectShootIfQuotaExceeded) ValidateInitialization() error {
	if h.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	if h.cloudProfileLister == nil {
		return errors.New("missing cloudProfile lister")
	}
	return nil
}

// Admit ensures that the object in-flight is of kind Shoot.
// In addition it checks that the request resources are within the quota limits.
func (h *RejectShootIfQuotaExceeded) Admit(a admission.Attributes) error {
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

	// TODO: implement quota limit check logic here
	if false {
		return admission.NewForbidden(a, fmt.Errorf("quota limits exceeded for %v", shoot.Name))
	}

	return nil
}
