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

package resourcereferencemanager

import (
	"errors"
	"fmt"
	"io"

	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authorization/authorizer"
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
	authorizer          authorizer.Authorizer
	secretLister        kubecorev1listers.SecretLister
	cloudProfileLister  gardenlisters.CloudProfileLister
	seedLister          gardenlisters.SeedLister
	secretBindingLister gardenlisters.SecretBindingLister
	quotaLister         gardenlisters.QuotaLister
}

var _ = admissioninitializer.WantsInternalGardenInformerFactory(&ReferenceManager{})
var _ = admissioninitializer.WantsKubeInformerFactory(&ReferenceManager{})
var _ = admissioninitializer.WantsAuthorizer(&ReferenceManager{})

// New creates a new ReferenceManager admission plugin.
func New() (*ReferenceManager, error) {
	return &ReferenceManager{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// SetAuthorizer gets the authorizer.
func (r *ReferenceManager) SetAuthorizer(authorizer authorizer.Authorizer) {
	r.authorizer = authorizer
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (r *ReferenceManager) SetInternalGardenInformerFactory(f gardeninformers.SharedInformerFactory) {
	r.cloudProfileLister = f.Garden().InternalVersion().CloudProfiles().Lister()
	r.seedLister = f.Garden().InternalVersion().Seeds().Lister()
	r.secretBindingLister = f.Garden().InternalVersion().SecretBindings().Lister()
	r.quotaLister = f.Garden().InternalVersion().Quotas().Lister()
}

// SetKubeInformerFactory gets Lister from SharedInformerFactory.
func (r *ReferenceManager) SetKubeInformerFactory(f kubeinformers.SharedInformerFactory) {
	r.secretLister = f.Core().V1().Secrets().Lister()
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (r *ReferenceManager) ValidateInitialization() error {
	if r.authorizer == nil {
		return errors.New("missing authorizer")
	}
	if r.secretLister == nil {
		return errors.New("missing secret lister")
	}
	if r.cloudProfileLister == nil {
		return errors.New("missing cloud profile lister")
	}
	if r.seedLister == nil {
		return errors.New("missing seed lister")
	}
	if r.secretBindingLister == nil {
		return errors.New("missing secret binding lister")
	}
	if r.quotaLister == nil {
		return errors.New("missing quota lister")
	}
	return nil
}

func skipVerification(operation admission.Operation, metadata metav1.ObjectMeta) bool {
	return operation == admission.Update && metadata.DeletionTimestamp != nil
}

// Admit ensures that referenced resources do actually exist.
func (r *ReferenceManager) Admit(a admission.Attributes) error {
	// Wait until the caches have been synced
	if !r.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	var (
		err       error
		operation = a.GetOperation()
	)

	switch a.GetKind().GroupKind() {
	case garden.Kind("SecretBinding"):
		binding, ok := a.GetObject().(*garden.SecretBinding)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into SecretBinding object")
		}
		if skipVerification(operation, binding.ObjectMeta) {
			return nil
		}
		err = r.ensureSecretBindingReferences(a, binding)

	case garden.Kind("Seed"):
		seed, ok := a.GetObject().(*garden.Seed)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into Seed object")
		}
		if skipVerification(operation, seed.ObjectMeta) {
			return nil
		}
		err = r.ensureSeedReferences(seed)

	case garden.Kind("Shoot"):
		shoot, ok := a.GetObject().(*garden.Shoot)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into Shoot object")
		}
		if skipVerification(operation, shoot.ObjectMeta) {
			return nil
		}
		err = r.ensureShootReferences(shoot)
	}

	if err != nil {
		return admission.NewForbidden(a, err)
	}
	return nil
}

func (r *ReferenceManager) ensureSecretBindingReferences(attributes admission.Attributes, binding *garden.SecretBinding) error {
	readAttributes := authorizer.AttributesRecord{
		User:            attributes.GetUserInfo(),
		Verb:            "get",
		APIGroup:        "",
		APIVersion:      "v1",
		Resource:        "secrets",
		Namespace:       binding.SecretRef.Namespace,
		Name:            binding.SecretRef.Name,
		ResourceRequest: true,
	}
	if decision, _, _ := r.authorizer.Authorize(readAttributes); decision != authorizer.DecisionAllow {
		return errors.New("SecretBinding cannot reference a secret you are not allowed to read")
	}

	if _, err := r.secretLister.Secrets(binding.SecretRef.Namespace).Get(binding.SecretRef.Name); err != nil {
		return err
	}

	var (
		secretQuotaCount  int
		projectQuotaCount int
	)

	for _, quotaRef := range binding.Quotas {
		readAttributes := authorizer.AttributesRecord{
			User:            attributes.GetUserInfo(),
			Verb:            "get",
			APIGroup:        gardenv1beta1.SchemeGroupVersion.Group,
			APIVersion:      gardenv1beta1.SchemeGroupVersion.Version,
			Resource:        "quotas",
			Subresource:     "",
			Namespace:       quotaRef.Namespace,
			Name:            quotaRef.Name,
			ResourceRequest: true,
			Path:            "",
		}
		if decision, _, _ := r.authorizer.Authorize(readAttributes); decision != authorizer.DecisionAllow {
			return errors.New("SecretBinding cannot reference a quota you are not allowed to read")
		}

		quota, err := r.quotaLister.Quotas(quotaRef.Namespace).Get(quotaRef.Name)
		if err != nil {
			return err
		}

		if quota.Spec.Scope == garden.QuotaScopeProject {
			projectQuotaCount++
		}
		if quota.Spec.Scope == garden.QuotaScopeSecret {
			secretQuotaCount++
		}
		if projectQuotaCount > 1 || secretQuotaCount > 1 {
			return fmt.Errorf("Only one quota per scope (%s or %s) can be assigned", garden.QuotaScopeProject, garden.QuotaScopeSecret)
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

	if _, err := r.secretBindingLister.SecretBindings(shoot.Namespace).Get(shoot.Spec.Cloud.SecretBindingRef.Name); err != nil {
		return err
	}

	return nil
}
