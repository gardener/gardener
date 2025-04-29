// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"errors"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"

	"github.com/gardener/gardener/pkg/apis/core"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	gardensecurityinformers "github.com/gardener/gardener/pkg/client/security/informers/externalversions"
	gardensecurityv1alpha1listers "github.com/gardener/gardener/pkg/client/security/listers/security/v1alpha1"
	plugin "github.com/gardener/gardener/plugin/pkg"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameSeedValidator, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ValidateSeed contains listers and admission handler.
type ValidateSeed struct {
	*admission.Handler
	seedLister             gardencorev1beta1listers.SeedLister
	shootLister            gardencorev1beta1listers.ShootLister
	workloadIdentityLister gardensecurityv1alpha1listers.WorkloadIdentityLister
	secretLister           kubecorev1listers.SecretLister
	readyFunc              admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreInformerFactory(&ValidateSeed{})
	_ = admissioninitializer.WantsSecurityInformerFactory(&ValidateSeed{})
	_ = admissioninitializer.WantsKubeInformerFactory(&ValidateSeed{})

	readyFuncs []admission.ReadyFunc
)

// New creates a new ValidateSeed admission plugin.
func New() (*ValidateSeed, error) {
	return &ValidateSeed{
		Handler: admission.NewHandler(admission.Delete, admission.Update, admission.Create),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (v *ValidateSeed) AssignReadyFunc(f admission.ReadyFunc) {
	v.readyFunc = f
	v.SetReadyFunc(f)
}

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (v *ValidateSeed) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	seedInformer := f.Core().V1beta1().Seeds()
	v.seedLister = seedInformer.Lister()

	shootInformer := f.Core().V1beta1().Shoots()
	v.shootLister = shootInformer.Lister()

	backupBucketInformer := f.Core().V1beta1().BackupBuckets()

	readyFuncs = append(readyFuncs, seedInformer.Informer().HasSynced, shootInformer.Informer().HasSynced, backupBucketInformer.Informer().HasSynced)
}

// SetSecurityInformerFactory gets Lister from SharedInformerFactory.
func (v *ValidateSeed) SetSecurityInformerFactory(f gardensecurityinformers.SharedInformerFactory) {
	wiInformer := f.Security().V1alpha1().WorkloadIdentities()
	v.workloadIdentityLister = wiInformer.Lister()

	readyFuncs = append(readyFuncs, wiInformer.Informer().HasSynced)
}

// SetKubeInformerFactory gets Lister from SharedInformerFactory.
func (v *ValidateSeed) SetKubeInformerFactory(f kubeinformers.SharedInformerFactory) {
	secretInformer := f.Core().V1().Secrets()
	v.secretLister = secretInformer.Lister()

	readyFuncs = append(readyFuncs, secretInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *ValidateSeed) ValidateInitialization() error {
	if v.seedLister == nil {
		return errors.New("missing seed lister")
	}
	if v.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	if v.workloadIdentityLister == nil {
		return errors.New("missing workloadidentity lister")
	}
	if v.secretLister == nil {
		return errors.New("missing secret lister")
	}
	return nil
}

var _ admission.ValidationInterface = &ValidateSeed{}

// Validate validates the Seed details against existing Shoots and BackupBuckets
func (v *ValidateSeed) Validate(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	// Wait until the caches have been synced
	if v.readyFunc == nil {
		v.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}

	if !v.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	// Ignore all kinds other than Seed
	if a.GetKind().GroupKind() != core.Kind("Seed") {
		return nil
	}

	// Ignore updates to status or other subresources
	if a.GetSubresource() != "" {
		return nil
	}

	switch a.GetOperation() {
	case admission.Create:
		return v.validateSeedCreate(a)
	case admission.Update:
		return v.validateSeedUpdate(a)
	case admission.Delete:
		return v.validateSeedDeletion(a)
	}

	return nil
}

func (v *ValidateSeed) validateSeedUpdate(a admission.Attributes) error {
	oldSeed, newSeed, err := getOldAndNewSeeds(a)
	if err != nil {
		return err
	}

	if err := admissionutils.ValidateZoneRemovalFromSeeds(&oldSeed.Spec, &newSeed.Spec, newSeed.Name, v.shootLister, "Seed"); err != nil {
		return err
	}

	return v.validateBackupCredentialsRef(a, newSeed)
}

func (v *ValidateSeed) validateSeedCreate(a admission.Attributes) error {
	seed, ok := a.GetObject().(*core.Seed)
	if !ok {
		return apierrors.NewInternalError(errors.New("failed to convert resource into Seed object"))
	}

	return v.validateBackupCredentialsRef(a, seed)
}

func (v *ValidateSeed) validateSeedDeletion(a admission.Attributes) error {
	seedName := a.GetName()

	shoots, err := v.shootLister.List(labels.Everything())
	if err != nil {
		return apierrors.NewInternalError(err)
	}

	if admissionutils.IsSeedUsedByShoot(seedName, shoots) {
		return admission.NewForbidden(a, fmt.Errorf("cannot delete seed %s since it is still used by shoot(s)", seedName))
	}
	return nil
}

func getOldAndNewSeeds(attrs admission.Attributes) (*core.Seed, *core.Seed, error) {
	var (
		oldSeed, newSeed *core.Seed
		ok               bool
	)

	if oldSeed, ok = attrs.GetOldObject().(*core.Seed); !ok {
		return nil, nil, apierrors.NewInternalError(errors.New("failed to convert old resource into Seed object"))
	}

	if newSeed, ok = attrs.GetObject().(*core.Seed); !ok {
		return nil, nil, apierrors.NewInternalError(errors.New("failed to convert new resource into Seed object"))
	}

	return oldSeed, newSeed, nil
}

func (v *ValidateSeed) validateBackupCredentialsRef(attrs admission.Attributes, seed *core.Seed) error {
	if seed.Spec.Backup == nil {
		return nil
	}

	backup := seed.Spec.Backup
	switch {
	case backup.CredentialsRef.APIVersion == securityv1alpha1.SchemeGroupVersion.String() &&
		backup.CredentialsRef.Kind == "WorkloadIdentity":
		workloadIdentity, err := v.workloadIdentityLister.WorkloadIdentities(backup.CredentialsRef.Namespace).Get(backup.CredentialsRef.Name)
		if err != nil {
			return apierrors.NewInternalError(err)
		}

		if seedBackupType, workloadIdentityType := backup.Provider, workloadIdentity.Spec.TargetSystem.Type; seedBackupType != workloadIdentityType {
			return admission.NewForbidden(attrs, fmt.Errorf("seed using backup of type %q cannot use WorkloadIdentity of type %q", seedBackupType, workloadIdentityType))
		}
	case backup.CredentialsRef.APIVersion == corev1.SchemeGroupVersion.String() &&
		backup.CredentialsRef.Kind == "Secret":
		_, err := v.secretLister.Secrets(backup.CredentialsRef.Namespace).Get(backup.CredentialsRef.Name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return admission.NewForbidden(attrs, fmt.Errorf("it is not allowed to reference a non-existent secret: %w", err))
			}
			return apierrors.NewInternalError(err)
		}
	default:
		return apierrors.NewBadRequest("unsupported credentials reference: backup config is referencing neither a Secret nor a WorkloadIdentity")
	}

	return nil
}
