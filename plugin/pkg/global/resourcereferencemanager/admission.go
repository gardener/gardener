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

package resourcereferencemanager

import (
	"errors"
	"fmt"
	"io"

	"github.com/gardener/gardener/pkg/apis/garden"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ResourceReferenceManager"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ReferenceManager contains listers and and admission handler.
type ReferenceManager struct {
	*admission.Handler
	secretLister               kubecorev1listers.SecretLister
	cloudProfileLister         gardenlisters.CloudProfileLister
	seedLister                 gardenlisters.SeedLister
	privateSecretBindingLister gardenlisters.PrivateSecretBindingLister
	crossSecretBindingLister   gardenlisters.CrossSecretBindingLister
	quotaLister                gardenlisters.QuotaLister
}

var _ = admissioninitializer.WantsInternalGardenInformerFactory(&ReferenceManager{})
var _ = admissioninitializer.WantsKubeInformerFactory(&ReferenceManager{})

// New creates a new ReferenceManager admission plugin.
func New() (*ReferenceManager, error) {
	return &ReferenceManager{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (r *ReferenceManager) SetInternalGardenInformerFactory(f gardeninformers.SharedInformerFactory) {
	r.cloudProfileLister = f.Garden().InternalVersion().CloudProfiles().Lister()
	r.seedLister = f.Garden().InternalVersion().Seeds().Lister()
	r.privateSecretBindingLister = f.Garden().InternalVersion().PrivateSecretBindings().Lister()
	r.crossSecretBindingLister = f.Garden().InternalVersion().CrossSecretBindings().Lister()
	r.quotaLister = f.Garden().InternalVersion().Quotas().Lister()
}

// SetKubeInformerFactory gets Lister from SharedInformerFactory.
func (r *ReferenceManager) SetKubeInformerFactory(f kubeinformers.SharedInformerFactory) {
	r.secretLister = f.Core().V1().Secrets().Lister()
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (r *ReferenceManager) ValidateInitialization() error {
	if r.secretLister == nil {
		return errors.New("missing secret lister")
	}
	if r.cloudProfileLister == nil {
		return errors.New("missing cloud profile lister")
	}
	if r.seedLister == nil {
		return errors.New("missing seed lister")
	}
	if r.privateSecretBindingLister == nil {
		return errors.New("missing private secret binding lister")
	}
	if r.crossSecretBindingLister == nil {
		return errors.New("missing cross secret binding lister")
	}
	if r.quotaLister == nil {
		return errors.New("missing quota lister")
	}
	return nil
}

// Admit ensures that referenced resources do actually exist.
func (r *ReferenceManager) Admit(a admission.Attributes) error {
	// Wait until the caches have been synced
	if !r.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	switch a.GetKind().GroupKind() {
	case garden.Kind("PrivateSecretBinding"):
		binding, ok := a.GetObject().(*garden.PrivateSecretBinding)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into PrivateSecretBinding object")
		}
		return r.ensurePrivateSecretBindingReferences(binding)

	case garden.Kind("CrossSecretBinding"):
		binding, ok := a.GetObject().(*garden.CrossSecretBinding)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into CrossSecretBinding object")
		}
		return r.ensureCrossSecretBindingReferences(binding)

	case garden.Kind("Seed"):
		seed, ok := a.GetObject().(*garden.Seed)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into Seed object")
		}
		return r.ensureSeedReferences(seed)

	case garden.Kind("Shoot"):
		shoot, ok := a.GetObject().(*garden.Shoot)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into Shoot object")
		}
		return r.ensureShootReferences(shoot)
	}

	return nil
}

func (r *ReferenceManager) ensurePrivateSecretBindingReferences(binding *garden.PrivateSecretBinding) error {
	return r.ensureBindingReferences(binding.Namespace, binding.SecretRef.Name, binding.Quotas)
}

func (r *ReferenceManager) ensureCrossSecretBindingReferences(binding *garden.CrossSecretBinding) error {
	return r.ensureBindingReferences(binding.SecretRef.Namespace, binding.SecretRef.Name, binding.Quotas)
}

func (r *ReferenceManager) ensureBindingReferences(secretNamespace, secretName string, quotaRefs []corev1.ObjectReference) error {
	if _, err := r.secretLister.Secrets(secretNamespace).Get(secretName); err != nil {
		return err
	}

	for _, quotaRef := range quotaRefs {
		if _, err := r.quotaLister.Quotas(quotaRef.Namespace).Get(quotaRef.Name); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReferenceManager) ensureSeedReferences(seed *garden.Seed) error {
	if _, err := r.cloudProfileLister.Get(seed.Spec.Cloud.Profile); err != nil {
		return err
	}

	if _, err := r.secretLister.Secrets(seed.Spec.SecretRef.Namespace).Get(seed.Spec.SecretRef.Name); err != nil {
		return err
	}

	return nil
}

func (r *ReferenceManager) ensureShootReferences(shoot *garden.Shoot) error {
	if _, err := r.cloudProfileLister.Get(shoot.Spec.Cloud.Profile); err != nil {
		return err
	}

	if shoot.Spec.Cloud.Seed != nil {
		if _, err := r.seedLister.Get(*shoot.Spec.Cloud.Seed); err != nil {
			return err
		}
	}

	switch shoot.Spec.Cloud.SecretBindingRef.Kind {
	case "PrivateSecretBinding":
		if _, err := r.privateSecretBindingLister.PrivateSecretBindings(shoot.Namespace).Get(shoot.Spec.Cloud.SecretBindingRef.Name); err != nil {
			return err
		}
	case "CrossSecretBinding":
		if _, err := r.crossSecretBindingLister.CrossSecretBindings(shoot.Namespace).Get(shoot.Spec.Cloud.SecretBindingRef.Name); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown secret binding reference kind '%s'", shoot.Spec.Cloud.SecretBindingRef.Kind)
	}

	return nil
}
