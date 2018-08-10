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
	"time"

	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
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
	kubeClient          kubernetes.Interface
	authorizer          authorizer.Authorizer
	secretLister        kubecorev1listers.SecretLister
	configMapLister     kubecorev1listers.ConfigMapLister
	cloudProfileLister  gardenlisters.CloudProfileLister
	seedLister          gardenlisters.SeedLister
	secretBindingLister gardenlisters.SecretBindingLister
	projectLister       gardenlisters.ProjectLister
	quotaLister         gardenlisters.QuotaLister
	readyFunc           admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalGardenInformerFactory(&ReferenceManager{})
	_ = admissioninitializer.WantsKubeInformerFactory(&ReferenceManager{})
	_ = admissioninitializer.WantsKubeClientset(&ReferenceManager{})
	_ = admissioninitializer.WantsAuthorizer(&ReferenceManager{})

	readyFuncs = []admission.ReadyFunc{}

	// MissingSecretWait is the time how long to wait for a missing secret before re-checking the cache
	// (and then doing a live lookup).
	MissingSecretWait = 50 * time.Millisecond
)

// New creates a new ReferenceManager admission plugin.
func New() (*ReferenceManager, error) {
	return &ReferenceManager{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (r *ReferenceManager) AssignReadyFunc(f admission.ReadyFunc) {
	r.readyFunc = f
	r.SetReadyFunc(f)
}

// SetAuthorizer gets the authorizer.
func (r *ReferenceManager) SetAuthorizer(authorizer authorizer.Authorizer) {
	r.authorizer = authorizer
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (r *ReferenceManager) SetInternalGardenInformerFactory(f gardeninformers.SharedInformerFactory) {
	seedInformer := f.Garden().InternalVersion().Seeds()
	r.seedLister = seedInformer.Lister()

	cloudProfileInformer := f.Garden().InternalVersion().CloudProfiles()
	r.cloudProfileLister = cloudProfileInformer.Lister()

	secretBindingInformer := f.Garden().InternalVersion().SecretBindings()
	r.secretBindingLister = secretBindingInformer.Lister()

	quotaInformer := f.Garden().InternalVersion().Quotas()
	r.quotaLister = quotaInformer.Lister()

	projectInformer := f.Garden().InternalVersion().Projects()
	r.projectLister = projectInformer.Lister()

	readyFuncs = append(readyFuncs, seedInformer.Informer().HasSynced, cloudProfileInformer.Informer().HasSynced, secretBindingInformer.Informer().HasSynced, quotaInformer.Informer().HasSynced, projectInformer.Informer().HasSynced)
}

// SetKubeInformerFactory gets Lister from SharedInformerFactory.
func (r *ReferenceManager) SetKubeInformerFactory(f kubeinformers.SharedInformerFactory) {
	secretInformer := f.Core().V1().Secrets()
	r.secretLister = secretInformer.Lister()

	configMapInformer := f.Core().V1().ConfigMaps()
	r.configMapLister = configMapInformer.Lister()

	readyFuncs = append(readyFuncs, secretInformer.Informer().HasSynced, configMapInformer.Informer().HasSynced)
}

// SetKubeClientset gets the clientset from the Kubernetes client.
func (r *ReferenceManager) SetKubeClientset(c kubernetes.Interface) {
	r.kubeClient = c
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (r *ReferenceManager) ValidateInitialization() error {
	if r.authorizer == nil {
		return errors.New("missing authorizer")
	}
	if r.secretLister == nil {
		return errors.New("missing secret lister")
	}
	if r.configMapLister == nil {
		return errors.New("missing configMap lister")
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
	if r.projectLister == nil {
		return errors.New("missing project lister")
	}
	return nil
}

func skipVerification(operation admission.Operation, metadata metav1.ObjectMeta) bool {
	return operation == admission.Update && metadata.DeletionTimestamp != nil
}

// Admit ensures that referenced resources do actually exist.
func (r *ReferenceManager) Admit(a admission.Attributes) error {
	// Wait until the caches have been synced
	if r.readyFunc == nil {
		r.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
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
		// Add createdBy annotation to Shoot
		if a.GetOperation() == admission.Create {
			annotations := shoot.Annotations
			if annotations == nil {
				annotations = map[string]string{}
			}
			annotations[common.GardenCreatedBy] = a.GetUserInfo().GetName()
			shoot.Annotations = annotations
		}
		err = r.ensureShootReferences(shoot)

	case garden.Kind("Project"):
		project, ok := a.GetObject().(*garden.Project)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into Project object")
		}
		if skipVerification(operation, project.ObjectMeta) {
			return nil
		}
		// Set createdBy field in Project
		if a.GetOperation() == admission.Create {
			project.Spec.CreatedBy = &rbacv1.Subject{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     rbacv1.UserKind,
				Name:     a.GetUserInfo().GetName(),
			}
			if project.Spec.Owner == nil {
				project.Spec.Owner = project.Spec.CreatedBy
			}
		}
		if a.GetOperation() == admission.Update {
			if createdBy, ok := project.Annotations[common.GardenCreatedBy]; ok {
				project.Spec.CreatedBy = &rbacv1.Subject{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     rbacv1.UserKind,
					Name:     createdBy,
				}
				delete(project.Annotations, common.GardenCreatedBy)
			}
		}

		if project.Spec.Owner != nil {
			ownerPartOfMember := false
			for _, member := range project.Spec.Members {
				if member == *project.Spec.Owner {
					ownerPartOfMember = true
				}
			}
			if !ownerPartOfMember {
				project.Spec.Members = append(project.Spec.Members, *project.Spec.Owner)
			}
		}
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

	if err := r.lookupSecret(binding.SecretRef.Namespace, binding.SecretRef.Name); err != nil {
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

	if err := r.lookupSecret(seed.Spec.SecretRef.Namespace, seed.Spec.SecretRef.Name); err != nil {
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

	if hasAuditPolicy(shoot.Spec.Kubernetes.KubeAPIServer) {
		if _, err := r.configMapLister.ConfigMaps(shoot.Namespace).Get(shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name); err != nil {
			return err
		}
	}

	return nil
}

func hasAuditPolicy(apiServerConfig *garden.KubeAPIServerConfig) bool {
	return apiServerConfig != nil &&
		apiServerConfig.AuditConfig != nil &&
		apiServerConfig.AuditConfig.AuditPolicy != nil &&
		apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef != nil &&
		len(apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef.Name) != 0
}

func (r *ReferenceManager) lookupSecret(namespace, name string) error {
	// First try to detect the secret in the cache.
	var err error

	_, err = r.secretLister.Secrets(namespace).Get(name)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	// Second try to detect the secret in the cache after the first try failed.
	// Give the cache time to observe the secret before rejecting a create.
	// This helps when creating a secret and immediately creating a secretbinding referencing it.
	time.Sleep(MissingSecretWait)
	_, err = r.secretLister.Secrets(namespace).Get(name)
	switch {
	case apierrors.IsNotFound(err):
		// no-op
	case err != nil:
		return err
	default:
		return nil
	}

	// Third try to detect the secret, now by doing a live lookup instead of relying on the cache.
	if _, err := r.kubeClient.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{}); err != nil {
		return err
	}

	return nil
}
