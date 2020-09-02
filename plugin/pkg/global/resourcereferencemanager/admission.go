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
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	corelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	clientkubernetes "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/plugin/pkg/utils"

	"github.com/hashicorp/go-multierror"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/dynamic"
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
	dynamicClient       dynamic.Interface
	authorizer          authorizer.Authorizer
	secretLister        kubecorev1listers.SecretLister
	configMapLister     kubecorev1listers.ConfigMapLister
	cloudProfileLister  corelisters.CloudProfileLister
	seedLister          corelisters.SeedLister
	shootLister         corelisters.ShootLister
	secretBindingLister corelisters.SecretBindingLister
	projectLister       corelisters.ProjectLister
	quotaLister         corelisters.QuotaLister
	readyFunc           admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalCoreInformerFactory(&ReferenceManager{})
	_ = admissioninitializer.WantsKubeInformerFactory(&ReferenceManager{})
	_ = admissioninitializer.WantsKubeClientset(&ReferenceManager{})
	_ = admissioninitializer.WantsDynamicClient(&ReferenceManager{})
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

// SetInternalCoreInformerFactory gets Lister from SharedInformerFactory.
func (r *ReferenceManager) SetInternalCoreInformerFactory(f coreinformers.SharedInformerFactory) {
	seedInformer := f.Core().InternalVersion().Seeds()
	r.seedLister = seedInformer.Lister()

	shootInformer := f.Core().InternalVersion().Shoots()
	r.shootLister = shootInformer.Lister()

	cloudProfileInformer := f.Core().InternalVersion().CloudProfiles()
	r.cloudProfileLister = cloudProfileInformer.Lister()

	secretBindingInformer := f.Core().InternalVersion().SecretBindings()
	r.secretBindingLister = secretBindingInformer.Lister()

	quotaInformer := f.Core().InternalVersion().Quotas()
	r.quotaLister = quotaInformer.Lister()

	projectInformer := f.Core().InternalVersion().Projects()
	r.projectLister = projectInformer.Lister()

	readyFuncs = append(readyFuncs, seedInformer.Informer().HasSynced, shootInformer.Informer().HasSynced, cloudProfileInformer.Informer().HasSynced, secretBindingInformer.Informer().HasSynced, quotaInformer.Informer().HasSynced, projectInformer.Informer().HasSynced)
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

// SetDynamicClient sets the dynamic client.
func (r *ReferenceManager) SetDynamicClient(c dynamic.Interface) {
	r.dynamicClient = c
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
	if r.shootLister == nil {
		return errors.New("missing shoot lister")
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

// Admit ensures that referenced resources do actually exist.
func (r *ReferenceManager) Admit(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
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
	case core.Kind("SecretBinding"):
		binding, ok := a.GetObject().(*core.SecretBinding)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into SecretBinding object")
		}
		if utils.SkipVerification(operation, binding.ObjectMeta) {
			return nil
		}
		err = r.ensureSecretBindingReferences(ctx, a, binding)

	case core.Kind("Seed"):
		seed, ok := a.GetObject().(*core.Seed)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into Seed object")
		}
		if utils.SkipVerification(operation, seed.ObjectMeta) {
			return nil
		}
		err = r.ensureSeedReferences(ctx, seed)

		if operation == admission.Update {
			oldSeed, ok := a.GetOldObject().(*core.Seed)
			if !ok {
				return apierrors.NewBadRequest("could not convert old resource into Seed object")
			}

			if oldSeed.Spec.Settings.ShootDNS.Enabled != seed.Spec.Settings.ShootDNS.Enabled {
				shootList, err2 := r.shootLister.List(labels.Everything())
				if err2 != nil {
					return err2
				}

				for _, shoot := range shootList {
					if shoot.Spec.SeedName == nil || *shoot.Spec.SeedName != seed.Name {
						continue
					}

					err = errors.New("may not change shoot DNS enablement setting when shoots are still referencing to a seed")
				}
			}
		}

	case core.Kind("Shoot"):
		shoot, ok := a.GetObject().(*core.Shoot)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into Shoot object")
		}
		if utils.SkipVerification(operation, shoot.ObjectMeta) {
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
		err = r.ensureShootReferences(ctx, a, shoot)

	case core.Kind("Project"):
		project, ok := a.GetObject().(*core.Project)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into Project object")
		}
		if utils.SkipVerification(operation, project.ObjectMeta) {
			return nil
		}
		// Set createdBy field in Project
		switch a.GetOperation() {
		case admission.Create:
			project.Spec.CreatedBy = &rbacv1.Subject{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     rbacv1.UserKind,
				Name:     a.GetUserInfo().GetName(),
			}

			if project.Spec.Owner == nil {
				owner := project.Spec.CreatedBy

			outer:
				for _, member := range project.Spec.Members {
					for _, role := range member.Roles {
						if role == core.ProjectMemberOwner {
							owner = member.Subject.DeepCopy()
							break outer
						}
					}
				}

				project.Spec.Owner = owner
			}

			err = r.ensureProjectNamespace(project)
		case admission.Update:
			oldProject, ok := a.GetOldObject().(*core.Project)
			if !ok {
				return apierrors.NewBadRequest("could not convert old resource into Project object")
			}
			if oldProject.Spec.Namespace == nil && project.Spec.Namespace != nil {
				err = r.ensureProjectNamespace(project)
			}
		}

		if project.Spec.Owner != nil {
			ownerIsMember := false
			for _, member := range project.Spec.Members {
				if member.Subject == *project.Spec.Owner {
					ownerIsMember = true
				}
			}
			if !ownerIsMember {
				project.Spec.Members = append(project.Spec.Members, core.ProjectMember{
					Subject: *project.Spec.Owner,
					Roles: []string{
						core.ProjectMemberAdmin,
						core.ProjectMemberOwner,
					},
				})
			}
		}
	case core.Kind("CloudProfile"):
		cloudProfile, ok := a.GetObject().(*core.CloudProfile)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into CloudProfile object")
		}
		if utils.SkipVerification(operation, cloudProfile.ObjectMeta) {
			return nil
		}
		if a.GetOperation() == admission.Update {
			oldCloudProfile, ok := a.GetOldObject().(*core.CloudProfile)
			if !ok {
				return apierrors.NewBadRequest("could not convert old resource into CloudProfile object")
			}

			// getting Kubernetes versions that have been removed from the CloudProfile
			removedKubernetesVersions := sets.StringKeySet(helper.GetRemovedVersions(oldCloudProfile.Spec.Kubernetes.Versions, cloudProfile.Spec.Kubernetes.Versions))

			// getting Machine image versions that have been removed from the CloudProfile
			removedMachineImageVersions := map[string]sets.String{}
			for _, oldImage := range oldCloudProfile.Spec.MachineImages {
				imageFound := false
				for _, newImage := range cloudProfile.Spec.MachineImages {
					if oldImage.Name == newImage.Name {
						imageFound = true
						removedMachineImageVersions[oldImage.Name] = sets.StringKeySet(
							helper.GetRemovedVersions(
								helper.ToExpirableVersions(oldImage.Versions),
								helper.ToExpirableVersions(newImage.Versions),
							),
						)
					}
				}
				if !imageFound {
					for _, version := range oldImage.Versions {
						if removedMachineImageVersions[oldImage.Name] == nil {
							removedMachineImageVersions[oldImage.Name] = sets.NewString()
						}
						removedMachineImageVersions[oldImage.Name] = removedMachineImageVersions[oldImage.Name].Insert(version.Version)
					}
				}
			}

			if len(removedKubernetesVersions) > 0 || len(removedMachineImageVersions) > 0 {
				shootList, err1 := r.shootLister.List(labels.Everything())
				if err1 != nil {
					return apierrors.NewInternalError(fmt.Errorf("could not list shoots to verify that Kubernetes and/or Machine image version can be removed: %v", err1))
				}

				var (
					channel = make(chan error)
					wg      sync.WaitGroup
				)
				wg.Add(len(shootList))

				for _, s := range shootList {
					if s.Spec.CloudProfileName != cloudProfile.Name {
						wg.Done()
						continue
					}

					go func(shoot *core.Shoot) {
						defer wg.Done()

						if removedKubernetesVersions.Has(shoot.Spec.Kubernetes.Version) {
							channel <- fmt.Errorf("unable to delete Kubernetes version %q from CloudProfile %q - version is still in use by shoot '%s/%s'", shoot.Spec.Kubernetes.Version, shoot.Spec.CloudProfileName, shoot.Namespace, shoot.Name)
						}
						for _, worker := range shoot.Spec.Provider.Workers {
							if worker.Machine.Image == nil {
								continue
							}
							// happens if Shoot runs an image that does not exist in the old CloudProfile - in this case: ignore
							if _, ok := removedMachineImageVersions[worker.Machine.Image.Name]; !ok {
								continue
							}

							if removedMachineImageVersions[worker.Machine.Image.Name].Has(worker.Machine.Image.Version) {
								channel <- fmt.Errorf("unable to delete Machine image version '%s/%s' from CloudProfile %q - version is still in use by shoot '%s/%s' by worker %q", worker.Machine.Image.Name, worker.Machine.Image.Version, shoot.Spec.CloudProfileName, shoot.Namespace, shoot.Name, worker.Name)
							}
						}
					}(s)
				}

				// close channel when wait group has 0 counter
				go func() {
					wg.Wait()
					close(channel)
				}()

				for channelResult := range channel {
					err = multierror.Append(err, channelResult)
				}
			}
		}
	}

	if err != nil {
		return admission.NewForbidden(a, err)
	}
	return nil
}

func (r *ReferenceManager) ensureProjectNamespace(project *core.Project) error {
	projects, err := r.projectLister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, p := range projects {
		if p.Spec.Namespace != nil && project.Spec.Namespace != nil && *p.Spec.Namespace == *project.Spec.Namespace && p.Name != project.Name {
			return fmt.Errorf("namespace %q is already used by another project", *project.Spec.Namespace)
		}
	}
	return nil
}

func (r *ReferenceManager) ensureSecretBindingReferences(ctx context.Context, attributes admission.Attributes, binding *core.SecretBinding) error {
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
	if decision, _, _ := r.authorizer.Authorize(ctx, readAttributes); decision != authorizer.DecisionAllow {
		return errors.New("SecretBinding cannot reference a secret you are not allowed to read")
	}

	if err := r.lookupSecret(ctx, binding.SecretRef.Namespace, binding.SecretRef.Name); err != nil {
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
			APIGroup:        gardencorev1beta1.SchemeGroupVersion.Group,
			APIVersion:      gardencorev1beta1.SchemeGroupVersion.Version,
			Resource:        "quotas",
			Subresource:     "",
			Namespace:       quotaRef.Namespace,
			Name:            quotaRef.Name,
			ResourceRequest: true,
			Path:            "",
		}
		if decision, _, _ := r.authorizer.Authorize(ctx, readAttributes); decision != authorizer.DecisionAllow {
			return errors.New("SecretBinding cannot reference a quota you are not allowed to read")
		}

		quota, err := r.quotaLister.Quotas(quotaRef.Namespace).Get(quotaRef.Name)
		if err != nil {
			return err
		}

		scope, err := helper.QuotaScope(quota.Spec.Scope)
		if err != nil {
			return err
		}

		if scope == "project" {
			projectQuotaCount++
		}
		if scope == "secret" {
			secretQuotaCount++
		}
		if projectQuotaCount > 1 || secretQuotaCount > 1 {
			return errors.New("only one quota per scope (project or secret) can be assigned")
		}
	}

	return nil
}

func (r *ReferenceManager) ensureSeedReferences(ctx context.Context, seed *core.Seed) error {
	if seed.Spec.SecretRef == nil {
		return nil
	}
	return r.lookupSecret(ctx, seed.Spec.SecretRef.Namespace, seed.Spec.SecretRef.Name)
}

func (r *ReferenceManager) ensureShootReferences(ctx context.Context, attributes admission.Attributes, shoot *core.Shoot) error {
	if _, err := r.cloudProfileLister.Get(shoot.Spec.CloudProfileName); err != nil {
		return err
	}

	if shoot.Spec.SeedName != nil {
		if _, err := r.seedLister.Get(*shoot.Spec.SeedName); err != nil {
			return err
		}
	}

	if _, err := r.secretBindingLister.SecretBindings(shoot.Namespace).Get(shoot.Spec.SecretBindingName); err != nil {
		return err
	}

	kubeAPIServer := shoot.Spec.Kubernetes.KubeAPIServer
	if hasAuditPolicy(kubeAPIServer) {
		auditPolicy, err := r.configMapLister.ConfigMaps(shoot.Namespace).Get(shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name)
		if err != nil {
			return err
		}
		kubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.ResourceVersion = auditPolicy.ResourceVersion
	}

	for _, resource := range shoot.Spec.Resources {
		// Get the APIResource for the current resource
		apiResource, err := r.getAPIResource(resource.ResourceRef.APIVersion, resource.ResourceRef.Kind)
		if err != nil {
			return err
		}
		if apiResource == nil {
			return fmt.Errorf("shoot resource reference %q could not be resolved for API resource with version %q and kind %q", resource.Name, resource.ResourceRef.APIVersion, resource.ResourceRef.Kind)
		}

		// Parse APIVersion to GroupVersion
		gv, err := schema.ParseGroupVersion(resource.ResourceRef.APIVersion)
		if err != nil {
			return err
		}

		// Check if the resource is namespaced
		if !apiResource.Namespaced {
			return fmt.Errorf("failed to resolve shoot resource reference %q. Cannot reference a resource that is not namespaced", resource.Name)
		}

		// Check if the user is allowed to read the resource
		readAttributes := authorizer.AttributesRecord{
			User:            attributes.GetUserInfo(),
			Verb:            "get",
			APIGroup:        gv.Group,
			APIVersion:      gv.Version,
			Resource:        apiResource.Name,
			Namespace:       shoot.Namespace,
			Name:            resource.ResourceRef.Name,
			ResourceRequest: true,
		}
		if decision, _, _ := r.authorizer.Authorize(ctx, readAttributes); decision != authorizer.DecisionAllow {
			return errors.New("shoot cannot reference a resource you are not allowed to read")
		}

		// Check if the resource actually exists
		if err := r.lookupResource(ctx, gv.WithResource(apiResource.Name), shoot.Namespace, resource.ResourceRef.Name); err != nil {
			return fmt.Errorf("failed to resolve shoot resource reference %q: %v", resource.Name, err)
		}
	}

	if shoot.Spec.DNS != nil && shoot.DeletionTimestamp == nil {
		for _, dnsProvider := range shoot.Spec.DNS.Providers {
			if dnsProvider.SecretName == nil {
				continue
			}
			if err := r.lookupSecret(ctx, shoot.Namespace, *dnsProvider.SecretName); err != nil {
				return fmt.Errorf("failed to reference DNS provider secret %v", err)
			}
		}
	}

	return nil
}

func hasAuditPolicy(apiServerConfig *core.KubeAPIServerConfig) bool {
	return apiServerConfig != nil &&
		apiServerConfig.AuditConfig != nil &&
		apiServerConfig.AuditConfig.AuditPolicy != nil &&
		apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef != nil &&
		len(apiServerConfig.AuditConfig.AuditPolicy.ConfigMapRef.Name) != 0
}

func (r *ReferenceManager) lookupSecret(ctx context.Context, namespace, name string) error {
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
	if _, err := r.kubeClient.CoreV1().Secrets(namespace).Get(ctx, name, clientkubernetes.DefaultGetOptions()); err != nil {
		return err
	}

	return nil
}

func (r *ReferenceManager) getAPIResource(groupVersion, kind string) (*metav1.APIResource, error) {
	resources, err := r.kubeClient.Discovery().ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		return nil, err
	}
	for _, apiResource := range resources.APIResources {
		if apiResource.Kind == kind {
			return &apiResource, nil
		}
	}
	return nil, nil
}

func (r *ReferenceManager) lookupResource(ctx context.Context, resource schema.GroupVersionResource, namespace, name string) error {
	if _, err := r.dynamicClient.Resource(resource).Namespace(namespace).Get(ctx, name, clientkubernetes.DefaultGetOptions()); err != nil {
		return err
	}
	return nil
}
