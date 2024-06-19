// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcereferencemanager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/dynamic"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/security"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	"github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	gardensecurityinformers "github.com/gardener/gardener/pkg/client/security/informers/externalversions"
	securityv1alpha1listers "github.com/gardener/gardener/pkg/client/security/listers/security/v1alpha1"
	seedmanagementinformers "github.com/gardener/gardener/pkg/client/seedmanagement/informers/externalversions"
	seedmanagementv1alpha1listers "github.com/gardener/gardener/pkg/client/seedmanagement/listers/seedmanagement/v1alpha1"
	plugin "github.com/gardener/gardener/plugin/pkg"
	"github.com/gardener/gardener/plugin/pkg/utils"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameResourceReferenceManager, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ReferenceManager contains listers and admission handler.
type ReferenceManager struct {
	*admission.Handler
	gardenCoreClient           versioned.Interface
	kubeClient                 kubernetes.Interface
	dynamicClient              dynamic.Interface
	authorizer                 authorizer.Authorizer
	secretLister               kubecorev1listers.SecretLister
	configMapLister            kubecorev1listers.ConfigMapLister
	backupBucketLister         gardencorev1beta1listers.BackupBucketLister
	cloudProfileLister         gardencorev1beta1listers.CloudProfileLister
	seedLister                 gardencorev1beta1listers.SeedLister
	shootLister                gardencorev1beta1listers.ShootLister
	secretBindingLister        gardencorev1beta1listers.SecretBindingLister
	credentialsBindingLister   securityv1alpha1listers.CredentialsBindingLister
	projectLister              gardencorev1beta1listers.ProjectLister
	quotaLister                gardencorev1beta1listers.QuotaLister
	controllerDeploymentLister gardencorev1beta1listers.ControllerDeploymentLister
	exposureClassLister        gardencorev1beta1listers.ExposureClassLister
	managedSeedLister          seedmanagementv1alpha1listers.ManagedSeedLister
	gardenletLister            seedmanagementv1alpha1listers.GardenletLister
	readyFunc                  admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreInformerFactory(&ReferenceManager{})
	_ = admissioninitializer.WantsSeedManagementInformerFactory(&ReferenceManager{})
	_ = admissioninitializer.WantsKubeInformerFactory(&ReferenceManager{})
	_ = admissioninitializer.WantsCoreClientSet(&ReferenceManager{})
	_ = admissioninitializer.WantsKubeClientset(&ReferenceManager{})
	_ = admissioninitializer.WantsDynamicClient(&ReferenceManager{})
	_ = admissioninitializer.WantsAuthorizer(&ReferenceManager{})

	readyFuncs []admission.ReadyFunc

	// MissingSecretWait is the time how long to wait for a missing secret before re-checking the cache
	// (and then doing a live lookup).
	MissingSecretWait = 50 * time.Millisecond
)

// New creates a new ReferenceManager admission plugin.
func New() (*ReferenceManager, error) {
	return &ReferenceManager{
		Handler: admission.NewHandler(admission.Create, admission.Update, admission.Delete),
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

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (r *ReferenceManager) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	seedInformer := f.Core().V1beta1().Seeds()
	r.seedLister = seedInformer.Lister()

	shootInformer := f.Core().V1beta1().Shoots()
	r.shootLister = shootInformer.Lister()

	backupBucketInformer := f.Core().V1beta1().BackupBuckets()
	r.backupBucketLister = backupBucketInformer.Lister()

	cloudProfileInformer := f.Core().V1beta1().CloudProfiles()
	r.cloudProfileLister = cloudProfileInformer.Lister()

	secretBindingInformer := f.Core().V1beta1().SecretBindings()
	r.secretBindingLister = secretBindingInformer.Lister()

	quotaInformer := f.Core().V1beta1().Quotas()
	r.quotaLister = quotaInformer.Lister()

	projectInformer := f.Core().V1beta1().Projects()
	r.projectLister = projectInformer.Lister()

	controllerDeploymentInformer := f.Core().V1beta1().ControllerDeployments()
	r.controllerDeploymentLister = controllerDeploymentInformer.Lister()

	exposureClassInformer := f.Core().V1beta1().ExposureClasses()
	r.exposureClassLister = exposureClassInformer.Lister()

	readyFuncs = append(readyFuncs,
		seedInformer.Informer().HasSynced,
		shootInformer.Informer().HasSynced,
		backupBucketInformer.Informer().HasSynced,
		cloudProfileInformer.Informer().HasSynced,
		secretBindingInformer.Informer().HasSynced,
		quotaInformer.Informer().HasSynced,
		projectInformer.Informer().HasSynced,
		controllerDeploymentInformer.Informer().HasSynced,
		exposureClassInformer.Informer().HasSynced)
}

// SetSeedManagementInformerFactory gets Lister from SharedInformerFactory.
func (r *ReferenceManager) SetSeedManagementInformerFactory(f seedmanagementinformers.SharedInformerFactory) {
	managedSeedInformer := f.Seedmanagement().V1alpha1().ManagedSeeds()
	r.managedSeedLister = managedSeedInformer.Lister()

	gardenletInformer := f.Seedmanagement().V1alpha1().Gardenlets()
	r.gardenletLister = gardenletInformer.Lister()

	readyFuncs = append(readyFuncs, managedSeedInformer.Informer().HasSynced, gardenletInformer.Informer().HasSynced)
}

// SetKubeInformerFactory gets Lister from SharedInformerFactory.
func (r *ReferenceManager) SetKubeInformerFactory(f kubeinformers.SharedInformerFactory) {
	secretInformer := f.Core().V1().Secrets()
	r.secretLister = secretInformer.Lister()

	configMapInformer := f.Core().V1().ConfigMaps()
	r.configMapLister = configMapInformer.Lister()

	readyFuncs = append(readyFuncs, secretInformer.Informer().HasSynced, configMapInformer.Informer().HasSynced)
}

// SetSecurityInformerFactory gets Lister from SharedInformerFactory.
func (r *ReferenceManager) SetSecurityInformerFactory(f gardensecurityinformers.SharedInformerFactory) {
	credentialsBindingInformer := f.Security().V1alpha1().CredentialsBindings()
	r.credentialsBindingLister = credentialsBindingInformer.Lister()

	readyFuncs = append(readyFuncs, credentialsBindingInformer.Informer().HasSynced)
}

// SetCoreClientSet sets the Gardener client.
func (r *ReferenceManager) SetCoreClientSet(c versioned.Interface) {
	r.gardenCoreClient = c
}

// SetKubeClientset sets the Kubernetes client.
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
	if r.backupBucketLister == nil {
		return errors.New("missing BackupBucket lister")
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
	if r.credentialsBindingLister == nil {
		return errors.New("missing credentials binding lister")
	}
	if r.quotaLister == nil {
		return errors.New("missing quota lister")
	}
	if r.projectLister == nil {
		return errors.New("missing project lister")
	}
	if r.exposureClassLister == nil {
		return errors.New("missing exposure class lister")
	}
	if r.gardenCoreClient == nil {
		return errors.New("missing gardener core client")
	}
	if r.managedSeedLister == nil {
		return errors.New("missing managed seed lister")
	}
	if r.gardenletLister == nil {
		return errors.New("missing gardenlet lister")
	}
	return nil
}

// Admit ensures that referenced resources do actually exist.
func (r *ReferenceManager) Admit(ctx context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
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

	if operation == admission.Delete && a.GetKind().GroupKind() != core.Kind("BackupBucket") {
		return nil
	}

	switch a.GetKind().GroupKind() {
	case core.Kind("SecretBinding"):
		binding, ok := a.GetObject().(*core.SecretBinding)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into SecretBinding object")
		}
		if utils.SkipVerification(operation, binding.ObjectMeta) {
			return nil
		}
		err = r.ensureBindingReferences(ctx, a, binding)

	case security.Kind("CredentialsBinding"):
		binding, ok := a.GetObject().(*security.CredentialsBinding)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into CredentialsBinding object")
		}
		if utils.SkipVerification(operation, binding.ObjectMeta) {
			return nil
		}
		err = r.ensureBindingReferences(ctx, a, binding)

	case core.Kind("Shoot"):
		var (
			oldShoot, shoot *core.Shoot
			ok              bool
		)

		shoot, ok = a.GetObject().(*core.Shoot)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into Shoot object")
		}
		if utils.SkipVerification(operation, shoot.ObjectMeta) {
			return nil
		}

		switch a.GetOperation() {
		case admission.Create:
			// Add createdBy annotation to Shoot
			annotations := shoot.Annotations
			if annotations == nil {
				annotations = map[string]string{}
			}
			annotations[v1beta1constants.GardenCreatedBy] = a.GetUserInfo().GetName()
			shoot.Annotations = annotations

			oldShoot = &core.Shoot{}
		case admission.Update:
			// skip verification if spec wasn't changed
			// this way we make sure, that users can always annotate/label the shoot if the spec doesn't change
			oldShoot, ok = a.GetOldObject().(*core.Shoot)
			if !ok {
				return apierrors.NewBadRequest("could not convert old resource into Shoot object")
			}
			if reflect.DeepEqual(oldShoot.Spec, shoot.Spec) {
				return nil
			}
		}
		err = r.ensureShootReferences(ctx, a, oldShoot, shoot)

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

	case core.Kind("BackupBucket"):
		if operation == admission.Delete {
			// The "delete endpoint" handler of the k8s.io/apiserver library calls the admission controllers
			// handling DELETECOLLECTION requests with empty resource names:
			// https://github.com/kubernetes/apiserver/blob/release-1.25/pkg/endpoints/handlers/delete.go#L271
			// Consequently, a.GetName() equals "". This is for the admission controllers to know that all
			// resources of this kind shall be deleted.
			// And for all DELETE requests, a.GetObject() will be nil:
			// https://github.com/kubernetes/apiserver/blob/release-1.25/pkg/endpoints/handlers/delete.go#L126
			if a.GetName() == "" {
				return r.validateBackupBucketDeleteCollection(ctx, a)
			} else {
				return r.validateBackupBucketDeletion(ctx, a)
			}
		} else {
			backupBucket, ok := a.GetObject().(*core.BackupBucket)
			if !ok {
				return apierrors.NewBadRequest("could not convert resource into BackupBucket object")
			}
			oldBackupBucket := &core.BackupBucket{}
			if operation == admission.Update {
				oldBackupBucket, ok = a.GetOldObject().(*core.BackupBucket)
				if !ok {
					return apierrors.NewBadRequest("could not convert old resource into BackupBucket object")
				}
			}

			err = r.ensureBackupBucketReferences(ctx, oldBackupBucket, backupBucket)
		}

	case core.Kind("BackupEntry"):
		backupEntry, ok := a.GetObject().(*core.BackupEntry)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into BackupEntry object")
		}
		oldBackupEntry := &core.BackupEntry{}
		if operation == admission.Update {
			oldBackupEntry, ok = a.GetOldObject().(*core.BackupEntry)
			if !ok {
				return apierrors.NewBadRequest("could not convert old resource into BackupEntry object")
			}
		}
		err = r.ensureBackupEntryReferences(oldBackupEntry, backupEntry)

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
			removedMachineImageVersions := map[string]sets.Set[string]{}
			for _, oldImage := range oldCloudProfile.Spec.MachineImages {
				imageFound := false
				for _, newImage := range cloudProfile.Spec.MachineImages {
					if oldImage.Name == newImage.Name {
						imageFound = true
						removedMachineImageVersions[oldImage.Name] = sets.KeySet(
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
							removedMachineImageVersions[oldImage.Name] = sets.New[string]()
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
					if s.Spec.CloudProfileName != cloudProfile.Name || s.DeletionTimestamp != nil {
						wg.Done()
						continue
					}

					go func(shoot *gardencorev1beta1.Shoot) {
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

							if removedMachineImageVersions[worker.Machine.Image.Name].Has(*worker.Machine.Image.Version) {
								channel <- fmt.Errorf("unable to delete Machine image version '%s/%s' from CloudProfile %q - version is still in use by shoot '%s/%s' by worker %q", worker.Machine.Image.Name, *worker.Machine.Image.Version, shoot.Spec.CloudProfileName, shoot.Namespace, shoot.Name, worker.Name)
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

	case core.Kind("ControllerRegistration"):
		controllerRegistration, ok := a.GetObject().(*core.ControllerRegistration)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into ControllerRegistration object")
		}
		err = r.ensureControllerRegistrationReferences(ctx, controllerRegistration)

	case seedmanagement.Kind("Gardenlet"):
		gardenlet, ok := a.GetObject().(*seedmanagement.Gardenlet)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into Gardenlet object")
		}
		if utils.SkipVerification(operation, gardenlet.ObjectMeta) {
			return nil
		}
		if _, err := r.managedSeedLister.ManagedSeeds(gardenlet.Namespace).Get(gardenlet.Name); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed checking whether ManagedSeed object exists for Gardenlet %s/%s: %w", gardenlet.Namespace, gardenlet.Name, err)
		} else if err == nil {
			return fmt.Errorf("cannot create Gardenlet %s/%s since there is already a ManagedSeed object with the same name", gardenlet.Namespace, gardenlet.Name)
		}

	case seedmanagement.Kind("ManagedSeed"):
		managedSeed, ok := a.GetObject().(*seedmanagement.ManagedSeed)
		if !ok {
			return apierrors.NewBadRequest("could not convert resource into ManagedSeed object")
		}
		if utils.SkipVerification(operation, managedSeed.ObjectMeta) {
			return nil
		}
		if _, err := r.gardenletLister.Gardenlets(managedSeed.Namespace).Get(managedSeed.Name); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed checking whether Gardenlet object exists for ManagedSeed %s/%s: %w", managedSeed.Namespace, managedSeed.Name, err)
		} else if err == nil {
			return fmt.Errorf("cannot create ManagedSeed %s/%s since there is already a Gardenlet object with the same name", managedSeed.Namespace, managedSeed.Name)
		}
	}

	if err != nil {
		return admission.NewForbidden(a, err)
	}
	return nil
}

func (r *ReferenceManager) ensureControllerRegistrationReferences(ctx context.Context, ctrlReg *core.ControllerRegistration) error {
	deployment := ctrlReg.Spec.Deployment
	if ctrlReg.Spec.Deployment == nil {
		return nil
	}

	var refErrs error
	for _, reg := range deployment.DeploymentRefs {
		if err := r.lookupControllerDeployment(ctx, reg.Name); err != nil {
			refErrs = multierror.Append(refErrs, err)
		}
	}

	return refErrs
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

func (r *ReferenceManager) ensureBindingReferences(ctx context.Context, attributes admission.Attributes, binding runtime.Object) error {
	var (
		quotas          []corev1.ObjectReference
		secretNamespace string
		secretName      string
	)
	switch attributes.GetKind().GroupKind() {
	case core.Kind("SecretBinding"):
		b, ok := binding.(*core.SecretBinding)
		if !ok {
			return errors.New("failed to convert binding to SecretBinding")
		}
		quotas = b.Quotas
		secretNamespace = b.SecretRef.Namespace
		secretName = b.SecretRef.Name
	case security.Kind("CredentialsBinding"):
		// TODO(dimityrmirchev): This code should eventually handle
		// references to Workload Identity
		b, ok := binding.(*security.CredentialsBinding)
		if !ok {
			return errors.New("failed to convert binding to CredentialsBinding")
		}
		quotas = b.Quotas
		secretNamespace = b.CredentialsRef.Namespace
		secretName = b.CredentialsRef.Name
	default:
		return fmt.Errorf("%s is neither of kind SecretBinding nor CredentialsBinding", attributes.GetKind().GroupKind())
	}
	readAttributes := authorizer.AttributesRecord{
		User:            attributes.GetUserInfo(),
		Verb:            "get",
		APIGroup:        "",
		APIVersion:      "v1",
		Resource:        "secrets",
		Namespace:       secretNamespace,
		Name:            secretName,
		ResourceRequest: true,
	}
	if decision, _, _ := r.authorizer.Authorize(ctx, readAttributes); decision != authorizer.DecisionAllow {
		return fmt.Errorf("%s cannot reference a secret you are not allowed to read", binding.GetObjectKind().GroupVersionKind().Kind)
	}

	if err := r.lookupSecret(ctx, secretNamespace, secretName); err != nil {
		return err
	}

	var (
		secretQuotaCount  int
		projectQuotaCount int
	)

	for _, quotaRef := range quotas {
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
			return fmt.Errorf("%s cannot reference a quota you are not allowed to read", binding.GetObjectKind().GroupVersionKind().Kind)
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

func (r *ReferenceManager) ensureShootReferences(ctx context.Context, attributes admission.Attributes, oldShoot, shoot *core.Shoot) error {
	if !equality.Semantic.DeepEqual(oldShoot.Spec.CloudProfileName, shoot.Spec.CloudProfileName) {
		if _, err := r.cloudProfileLister.Get(shoot.Spec.CloudProfileName); err != nil {
			return err
		}
	}

	if !equality.Semantic.DeepEqual(oldShoot.Spec.SeedName, shoot.Spec.SeedName) {
		if shoot.Spec.SeedName != nil {
			if _, err := r.seedLister.Get(*shoot.Spec.SeedName); err != nil {
				return err
			}
		}
	}

	if shoot.Spec.SecretBindingName != nil && !equality.Semantic.DeepEqual(oldShoot.Spec.SecretBindingName, shoot.Spec.SecretBindingName) {
		if _, err := r.secretBindingLister.SecretBindings(shoot.Namespace).Get(*shoot.Spec.SecretBindingName); err != nil {
			return err
		}
	}

	if shoot.Spec.CredentialsBindingName != nil && !equality.Semantic.DeepEqual(oldShoot.Spec.CredentialsBindingName, shoot.Spec.CredentialsBindingName) {
		if _, err := r.credentialsBindingLister.CredentialsBindings(shoot.Namespace).Get(*shoot.Spec.CredentialsBindingName); err != nil {
			return err
		}
	}

	if !equality.Semantic.DeepEqual(oldShoot.Spec.ExposureClassName, shoot.Spec.ExposureClassName) && shoot.Spec.ExposureClassName != nil {
		if _, err := r.exposureClassLister.Get(*shoot.Spec.ExposureClassName); err != nil {
			return err
		}
	}

	if !equality.Semantic.DeepEqual(oldShoot.Spec.Resources, shoot.Spec.Resources) {
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
				return fmt.Errorf("failed to resolve shoot resource reference %q: %w", resource.Name, err)
			}
		}
	}

	if !equality.Semantic.DeepEqual(oldShoot.Spec.DNS, shoot.Spec.DNS) && shoot.Spec.DNS != nil && shoot.DeletionTimestamp == nil {
		for _, dnsProvider := range shoot.Spec.DNS.Providers {
			if dnsProvider.SecretName == nil {
				continue
			}
			if err := r.lookupSecret(ctx, shoot.Namespace, *dnsProvider.SecretName); err != nil {
				return fmt.Errorf("failed to reference DNS provider secret %w", err)
			}
		}
	}

	return nil
}

func (r *ReferenceManager) ensureBackupEntryReferences(oldBackupEntry, backupEntry *core.BackupEntry) error {
	if !equality.Semantic.DeepEqual(oldBackupEntry.Spec.SeedName, backupEntry.Spec.SeedName) {
		if backupEntry.Spec.SeedName != nil {
			if _, err := r.seedLister.Get(*backupEntry.Spec.SeedName); err != nil {
				return err
			}
		}
	}

	if !equality.Semantic.DeepEqual(oldBackupEntry.Spec.BucketName, backupEntry.Spec.BucketName) {
		if _, err := r.backupBucketLister.Get(backupEntry.Spec.BucketName); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReferenceManager) ensureBackupBucketReferences(ctx context.Context, oldBackupBucket, backupBucket *core.BackupBucket) error {
	if !equality.Semantic.DeepEqual(oldBackupBucket.Spec.SeedName, backupBucket.Spec.SeedName) {
		if backupBucket.Spec.SeedName != nil {
			if _, err := r.seedLister.Get(*backupBucket.Spec.SeedName); err != nil {
				return err
			}
		}
	}

	return r.lookupSecret(ctx, backupBucket.Spec.SecretRef.Namespace, backupBucket.Spec.SecretRef.Name)
}

func (r *ReferenceManager) validateBackupBucketDeleteCollection(ctx context.Context, a admission.Attributes) error {
	backupBucketList, err := r.gardenCoreClient.CoreV1beta1().BackupBuckets().List(ctx, metav1.ListOptions{LabelSelector: labels.Everything().String()})
	if err != nil {
		return err
	}

	for _, backupBucket := range backupBucketList.Items {
		if err := r.validateBackupBucketDeletion(ctx, utils.NewAttributesWithName(a, backupBucket.Name)); err != nil {
			return err
		}
	}

	return nil
}

func (r *ReferenceManager) validateBackupBucketDeletion(ctx context.Context, a admission.Attributes) error {
	backupEntryList, err := r.gardenCoreClient.CoreV1beta1().BackupEntries("").List(ctx, metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{core.BackupEntryBucketName: a.GetName()}).String(),
	})
	if err != nil {
		return err
	}

	associatedBackupEntries := make([]string, 0, len(backupEntryList.Items))
	for _, entry := range backupEntryList.Items {
		associatedBackupEntries = append(associatedBackupEntries, client.ObjectKeyFromObject(&entry).String())
	}

	if len(associatedBackupEntries) > 0 {
		return admission.NewForbidden(a, fmt.Errorf("cannot delete BackupBucket because BackupEntries are still referencing it, backupEntryNames: %s", strings.Join(associatedBackupEntries, ",")))
	}

	return nil
}

type getFn func(context.Context, string, string) (runtime.Object, error)

func lookupResource(ctx context.Context, namespace, name string, get getFn, fallbackGet getFn) error {
	// First try to detect the secret in the cache.
	var err error

	_, err = get(ctx, namespace, name)
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
	_, err = get(ctx, namespace, name)

	switch {
	case apierrors.IsNotFound(err):
		// no-op
	case err != nil:
		return err
	default:
		return nil
	}

	// Third try to detect the secret, now by doing a live lookup instead of relying on the cache.
	if _, err := fallbackGet(ctx, namespace, name); err != nil {
		return err
	}

	return nil
}

func (r *ReferenceManager) lookupSecret(ctx context.Context, namespace, name string) error {
	secretFromLister := func(_ context.Context, namespace, name string) (runtime.Object, error) {
		return r.secretLister.Secrets(namespace).Get(name)
	}

	secretFromClient := func(ctx context.Context, namespace, name string) (runtime.Object, error) {
		return r.kubeClient.CoreV1().Secrets(namespace).Get(ctx, name, kubernetesclient.DefaultGetOptions())
	}

	return lookupResource(ctx, namespace, name, secretFromLister, secretFromClient)
}

func (r *ReferenceManager) lookupControllerDeployment(ctx context.Context, name string) error {
	deploymentFromLister := func(_ context.Context, _, name string) (runtime.Object, error) {
		return r.controllerDeploymentLister.Get(name)
	}

	deploymentFromClient := func(ctx context.Context, _, name string) (runtime.Object, error) {
		return r.gardenCoreClient.CoreV1beta1().ControllerDeployments().Get(ctx, name, kubernetesclient.DefaultGetOptions())
	}

	return lookupResource(ctx, "", name, deploymentFromLister, deploymentFromClient)
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
	if _, err := r.dynamicClient.Resource(resource).Namespace(namespace).Get(ctx, name, kubernetesclient.DefaultGetOptions()); err != nil {
		return err
	}
	return nil
}
