// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"errors"
	"fmt"
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardensecurityinformers "github.com/gardener/gardener/pkg/client/security/informers/externalversions"
	gardensecurityv1alpha1listers "github.com/gardener/gardener/pkg/client/security/listers/security/v1alpha1"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameBackupBucketValidator, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ValidateBackupBucket contains listers and admission handler.
type ValidateBackupBucket struct {
	*admission.Handler
	workloadIdentityLister gardensecurityv1alpha1listers.WorkloadIdentityLister
	readyFunc              admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsSecurityInformerFactory(&ValidateBackupBucket{})

	readyFuncs []admission.ReadyFunc
)

// New creates a new ValidateBackupBucket admission plugin.
func New() (*ValidateBackupBucket, error) {
	return &ValidateBackupBucket{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (v *ValidateBackupBucket) AssignReadyFunc(f admission.ReadyFunc) {
	v.readyFunc = f
	v.SetReadyFunc(f)
}

// SetSecurityInformerFactory gets Lister from SharedInformerFactory.
func (v *ValidateBackupBucket) SetSecurityInformerFactory(f gardensecurityinformers.SharedInformerFactory) {
	wiInformer := f.Security().V1alpha1().WorkloadIdentities()
	v.workloadIdentityLister = wiInformer.Lister()

	readyFuncs = append(readyFuncs, wiInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *ValidateBackupBucket) ValidateInitialization() error {
	if v.workloadIdentityLister == nil {
		return errors.New("missing workloadidentity lister")
	}
	return nil
}

var _ admission.ValidationInterface = &ValidateBackupBucket{}

// Validate validates the BackupBucket details against existing Workload Identities
func (v *ValidateBackupBucket) Validate(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
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

	// Ignore all kinds other than BackupBucket
	if a.GetKind().GroupKind() != core.Kind("BackupBucket") {
		return nil
	}

	// Ignore updates to status or other subresources
	if a.GetSubresource() != "" {
		return nil
	}

	switch a.GetOperation() {
	case admission.Create:
		return v.validateBackupBucketCreate(a)
	case admission.Update:
		return v.validateBackupBucketUpdate(a)
	}

	return nil
}

func (v *ValidateBackupBucket) validateBackupBucketUpdate(a admission.Attributes) error {
	_, newBackupBucket, err := getOldAndNewBackupBuckets(a)
	if err != nil {
		return err
	}

	return v.validateCredentialsRef(a, newBackupBucket)
}

func (v *ValidateBackupBucket) validateBackupBucketCreate(a admission.Attributes) error {
	backupBucket, ok := a.GetObject().(*core.BackupBucket)
	if !ok {
		return apierrors.NewInternalError(errors.New("failed to convert resource into BackupBucket object"))
	}

	return v.validateCredentialsRef(a, backupBucket)
}

func getOldAndNewBackupBuckets(attrs admission.Attributes) (*core.BackupBucket, *core.BackupBucket, error) {
	var (
		oldBackupBucket, newBackupBucket *core.BackupBucket
		ok                               bool
	)

	if oldBackupBucket, ok = attrs.GetOldObject().(*core.BackupBucket); !ok {
		return nil, nil, apierrors.NewInternalError(errors.New("failed to convert old resource into BackupBucket object"))
	}

	if newBackupBucket, ok = attrs.GetObject().(*core.BackupBucket); !ok {
		return nil, nil, apierrors.NewInternalError(errors.New("failed to convert new resource into BackupBucket object"))
	}

	return oldBackupBucket, newBackupBucket, nil
}

func (v *ValidateBackupBucket) validateCredentialsRef(attrs admission.Attributes, backupBucket *core.BackupBucket) error {
	if backupBucket.Spec.CredentialsRef == nil {
		return nil
	}

	if backupBucket.Spec.CredentialsRef.APIVersion != securityv1alpha1.SchemeGroupVersion.String() || backupBucket.Spec.CredentialsRef.Kind != "WorkloadIdentity" {
		return nil
	}

	workloadIdentity, err := v.workloadIdentityLister.WorkloadIdentities(backupBucket.Spec.CredentialsRef.Namespace).Get(backupBucket.Spec.CredentialsRef.Name)
	if err != nil {
		return apierrors.NewInternalError(err)
	}

	if backupBucketProviderType, workloadIdentityType := backupBucket.Spec.Provider.Type, workloadIdentity.Spec.TargetSystem.Type; backupBucketProviderType != workloadIdentityType {
		return admission.NewForbidden(attrs, fmt.Errorf("BackupBucket using backup of type %q cannot use WorkloadIdentity of type %q", backupBucketProviderType, workloadIdentityType))
	}

	return nil
}
