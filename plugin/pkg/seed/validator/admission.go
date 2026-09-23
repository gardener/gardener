// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	gardensecurityinformers "github.com/gardener/gardener/pkg/client/security/informers/externalversions"
	gardensecurityv1alpha1listers "github.com/gardener/gardener/pkg/client/security/listers/security/v1alpha1"
	"github.com/gardener/gardener/pkg/features"
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

	shootLister            gardencorev1beta1listers.ShootLister
	secretLister           kubecorev1listers.SecretLister
	configMapLister        kubecorev1listers.ConfigMapLister
	workloadIdentityLister gardensecurityv1alpha1listers.WorkloadIdentityLister
	readyFunc              admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreInformerFactory(&ValidateSeed{})
	_ = admissioninitializer.WantsKubeInformerFactory(&ValidateSeed{})
	_ = admissioninitializer.WantsSecurityInformerFactory(&ValidateSeed{})

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
	shootInformer := f.Core().V1beta1().Shoots()
	v.shootLister = shootInformer.Lister()

	readyFuncs = append(readyFuncs, shootInformer.Informer().HasSynced)
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

	configMapInformer := f.Core().V1().ConfigMaps()
	v.configMapLister = configMapInformer.Lister()

	readyFuncs = append(readyFuncs, secretInformer.Informer().HasSynced)
	readyFuncs = append(readyFuncs, configMapInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *ValidateSeed) ValidateInitialization() error {
	if v.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	if v.configMapLister == nil {
		return errors.New("missing ConfigMap lister")
	}
	if v.secretLister == nil {
		return errors.New("missing Secret lister")
	}
	if v.workloadIdentityLister == nil {
		return errors.New("missing WorkloadIdentity lister")
	}
	return nil
}

var _ admission.ValidationInterface = (*ValidateSeed)(nil)

// Validate validates the Seed details against existing Shoots
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

	if metav1.HasLabel(oldSeed.ObjectMeta, v1beta1constants.LabelSelfHostedShootCluster) &&
		!metav1.HasLabel(newSeed.ObjectMeta, v1beta1constants.LabelSelfHostedShootCluster) {
		return admission.NewForbidden(a, fmt.Errorf("label %q cannot be removed from a Seed", v1beta1constants.LabelSelfHostedShootCluster))
	}

	if err := admissionutils.ValidateZoneRemovalFromSeeds(&oldSeed.Spec, &newSeed.Spec, newSeed.Name, v.shootLister, "Seed"); err != nil {
		return err
	}

	if err := admissionutils.ValidateInternalDomainChangeForSeed(&oldSeed.Spec, &newSeed.Spec, newSeed.Name, v.shootLister, "Seed"); err != nil {
		return err
	}

	if err := admissionutils.ValidateDefaultDomainsChangeForSeed(&oldSeed.Spec, &newSeed.Spec, newSeed.Name, v.shootLister, v.secretLister, "Seed"); err != nil {
		return err
	}

	if err := v.validateCredentialsRef(a, newSeed); err != nil {
		return err
	}

	if newSeed.DeletionTimestamp != nil && apiequality.Semantic.DeepEqual(oldSeed.Spec, newSeed.Spec) {
		return nil
	}

	return v.validateReferenceAllowlist(a, newSeed)
}

func (v *ValidateSeed) validateSeedCreate(a admission.Attributes) error {
	seed, ok := a.GetObject().(*core.Seed)
	if !ok {
		return apierrors.NewInternalError(errors.New("failed to convert resource into Seed object"))
	}

	if err := v.validateCredentialsRef(a, seed); err != nil {
		return err
	}

	return v.validateReferenceAllowlist(a, seed)
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

func (v *ValidateSeed) validateCredentialsRef(attrs admission.Attributes, seed *core.Seed) error {
	if seed.Spec.Backup == nil {
		return nil
	}

	if seed.Spec.Backup.CredentialsRef.APIVersion != securityv1alpha1.SchemeGroupVersion.String() || seed.Spec.Backup.CredentialsRef.Kind != "WorkloadIdentity" {
		return nil
	}

	workloadIdentity, err := v.workloadIdentityLister.WorkloadIdentities(seed.Spec.Backup.CredentialsRef.Namespace).Get(seed.Spec.Backup.CredentialsRef.Name)
	if err != nil {
		return apierrors.NewInternalError(err)
	}

	if seedBackupType, workloadIdentityType := seed.Spec.Backup.Provider, workloadIdentity.Spec.TargetSystem.Type; seedBackupType != workloadIdentityType {
		return admission.NewForbidden(attrs, fmt.Errorf("seed using backup of type %q cannot use WorkloadIdentity of type %q", seedBackupType, workloadIdentityType))
	}

	return nil
}

func (v *ValidateSeed) validateReferenceAllowlist(attrs admission.Attributes, seed *core.Seed) error {
	if !features.DefaultFeatureGate.Enabled(features.AllowlistSeedReferences) {
		return nil
	}

	if seed.Spec.Backup != nil {
		if ref := seed.Spec.Backup.CredentialsRef; ref != nil {
			if err := v.checkAllowlistAnnotation(attrs, seed.Name, ref.Namespace, ref.Name, ref.APIVersion, ref.Kind, "spec.backup.credentialsRef"); err != nil {
				return err
			}
		}
	}

	if seed.Spec.DNS.Provider != nil {
		if ref := seed.Spec.DNS.Provider.CredentialsRef; ref != nil {
			if err := v.checkAllowlistAnnotation(attrs, seed.Name, ref.Namespace, ref.Name, ref.APIVersion, ref.Kind, "spec.dns.provider.credentialsRef"); err != nil {
				return err
			}
		}
	}

	if seed.Spec.DNS.Internal != nil {
		ref := seed.Spec.DNS.Internal.CredentialsRef
		if err := v.checkAllowlistAnnotation(attrs, seed.Name, ref.Namespace, ref.Name, ref.APIVersion, ref.Kind, "spec.dns.internal.credentialsRef"); err != nil {
			return err
		}
	}

	for i, defaultDNS := range seed.Spec.DNS.Defaults {
		ref := defaultDNS.CredentialsRef
		if err := v.checkAllowlistAnnotation(attrs, seed.Name, ref.Namespace, ref.Name, ref.APIVersion, ref.Kind, fmt.Sprintf("spec.dns.defaults[%d].credentialsRef", i)); err != nil {
			return err
		}
	}

	for i, resource := range seed.Spec.Resources {
		ref := resource.ResourceRef
		if err := v.checkAllowlistAnnotation(attrs, seed.Name, v1beta1constants.GardenNamespace, ref.Name, ref.APIVersion, ref.Kind, fmt.Sprintf("spec.resources[%d].resourceRef", i)); err != nil {
			return err
		}
	}

	return nil
}

func (v *ValidateSeed) checkAllowlistAnnotation(attrs admission.Attributes, seedName, namespace, name, apiVersion, kind, fieldPath string) error {
	var annotations map[string]string

	switch {
	case apiVersion == corev1.SchemeGroupVersion.String() && kind == "Secret":
		obj, err := v.secretLister.Secrets(namespace).Get(name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return admission.NewForbidden(attrs, fmt.Errorf("%s: referenced Secret %s/%s not found or not allowlisted for seed %q", fieldPath, namespace, name, seedName))
			}
			return apierrors.NewInternalError(err)
		}
		annotations = obj.Annotations

	case apiVersion == corev1.SchemeGroupVersion.String() && kind == "ConfigMap":
		obj, err := v.configMapLister.ConfigMaps(namespace).Get(name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return admission.NewForbidden(attrs, fmt.Errorf("%s: referenced ConfigMap %s/%s not found or not allowlisted for seed %q", fieldPath, namespace, name, seedName))
			}
			return apierrors.NewInternalError(err)
		}
		annotations = obj.Annotations

	case apiVersion == securityv1alpha1.SchemeGroupVersion.String() && kind == "WorkloadIdentity":
		obj, err := v.workloadIdentityLister.WorkloadIdentities(namespace).Get(name)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return admission.NewForbidden(attrs, fmt.Errorf("%s: referenced WorkloadIdentity %s/%s not found or not allowlisted for seed %q", fieldPath, namespace, name, seedName))
			}
			return apierrors.NewInternalError(err)
		}
		annotations = obj.Annotations

	default:
		return nil
	}

	if !isSeedAllowlisted(annotations, seedName) {
		return admission.NewForbidden(attrs, fmt.Errorf("%s: referenced %s %s/%s is not allowlisted for seed %q (missing annotation %q with value %q or %q)",
			fieldPath, kind, namespace, name, seedName, v1beta1constants.AnnotationSeedNames, seedName, "*"))
	}

	return nil
}

func isSeedAllowlisted(annotations map[string]string, seedName string) bool {
	value, ok := annotations[v1beta1constants.AnnotationSeedNames]
	if !ok {
		return false
	}
	for _, token := range strings.Split(value, ",") {
		token = strings.TrimSpace(token)
		if token == "*" || token == seedName {
			return true
		}
	}
	return false
}
