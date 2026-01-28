// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dns

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strconv"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	plugin "github.com/gardener/gardener/plugin/pkg"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameShootDNS, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// DNS contains listers and admission handler.
type DNS struct {
	*admission.Handler

	secretLister  kubecorev1listers.SecretLister
	projectLister gardencorev1beta1listers.ProjectLister
	seedLister    gardencorev1beta1listers.SeedLister
	readyFunc     admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreInformerFactory(&DNS{})
	_ = admissioninitializer.WantsKubeInformerFactory(&DNS{})

	readyFuncs []admission.ReadyFunc
)

// New creates a new DNS admission plugin.
func New() (*DNS, error) {
	return &DNS{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (d *DNS) AssignReadyFunc(f admission.ReadyFunc) {
	d.readyFunc = f
	d.SetReadyFunc(f)
}

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (d *DNS) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	projectInformer := f.Core().V1beta1().Projects()
	d.projectLister = projectInformer.Lister()

	seedInformer := f.Core().V1beta1().Seeds()
	d.seedLister = seedInformer.Lister()

	readyFuncs = append(readyFuncs, projectInformer.Informer().HasSynced, seedInformer.Informer().HasSynced)
}

// SetKubeInformerFactory gets Lister from SharedInformerFactory.
func (d *DNS) SetKubeInformerFactory(f kubeinformers.SharedInformerFactory) {
	secretInformer := f.Core().V1().Secrets()
	d.secretLister = secretInformer.Lister()

	readyFuncs = append(readyFuncs, secretInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (d *DNS) ValidateInitialization() error {
	if d.secretLister == nil {
		return errors.New("missing secret lister")
	}
	if d.projectLister == nil {
		return errors.New("missing project lister")
	}
	if d.seedLister == nil {
		return errors.New("missing seed lister")
	}
	return nil
}

var (
	_ admission.MutationInterface   = (*DNS)(nil)
	_ admission.ValidationInterface = (*DNS)(nil)
)

// Admit tries to determine a DNS hosted zone for the Shoot's external domain.
func (d *DNS) Admit(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	if err := d.waitUntilReady(a); err != nil {
		return fmt.Errorf("err while waiting for ready: %w", err)
	}

	if shouldIgnore(a) {
		return nil
	}

	shoot, ok := a.GetObject().(*core.Shoot)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Shoot object")
	}

	defaultDomains, err := getDefaultDomains(d.secretLister, d.seedLister, shoot)
	if err != nil {
		return fmt.Errorf("error retrieving default domains: %s", err)
	}

	if a.GetOperation() == admission.Update {
		oldShoot, ok := a.GetOldObject().(*core.Shoot)
		if !ok {
			return apierrors.NewBadRequest("could not convert old resource into Shoot object")
		}

		if oldShoot.Spec.DNS != nil && shoot.Spec.DNS != nil {
			oldPrimaryProvider := helper.FindPrimaryDNSProvider(oldShoot.Spec.DNS.Providers)
			primaryProvider := helper.FindPrimaryDNSProvider(shoot.Spec.DNS.Providers)
			if oldPrimaryProvider != nil && primaryProvider == nil {
				// Since it was possible to apply shoots w/o a primary provider before, we have to re-add it here.
				for i, provider := range shoot.Spec.DNS.Providers {
					if reflect.DeepEqual(provider.Type, oldPrimaryProvider.Type) && reflect.DeepEqual(provider.CredentialsRef, oldPrimaryProvider.CredentialsRef) {
						shoot.Spec.DNS.Providers[i].Primary = ptr.To(true)
						break
					}
				}
			}
		}
	}

	if shoot.Spec.SeedName == nil {
		return nil
	}

	specPath := field.NewPath("spec")

	// Generate a Shoot domain if none is configured.
	if !helper.ShootUsesUnmanagedDNS(shoot) {
		if err := assignDefaultDomainIfNeeded(shoot, d.projectLister, defaultDomains); err != nil {
			return err
		}

		if !isShootDomainSet(shoot) {
			fieldErr := field.Required(specPath.Child("DNS"), fmt.Sprintf("shoot domain field .spec.dns.domain must be set if provider != %s", core.DNSUnmanaged))
			return apierrors.NewInvalid(a.GetKind().GroupKind(), shoot.Name, field.ErrorList{fieldErr})
		}
	}

	if shoot.Spec.DNS != nil {
		if err := setPrimaryDNSProvider(shoot, defaultDomains); err != nil {
			return err
		}
	}
	return nil
}

func (d *DNS) waitUntilReady(a admission.Attributes) error {
	// Wait until the caches have been synced
	if d.readyFunc == nil {
		d.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
	if !d.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	return nil
}

func shouldIgnore(a admission.Attributes) bool {
	// Ignore all kinds other than Shoot
	if a.GetKind().GroupKind() != core.Kind("Shoot") {
		return true
	}
	// Ignore updates to all subresources, except for binding.
	// Binding subresource is required because there are fields being set in the shoot
	// when it is scheduled and we want this plugin to be triggered.
	if a.GetSubresource() != "" && a.GetSubresource() != "binding" {
		return true
	}

	return false
}

func isShootDomainSet(shoot *core.Shoot) bool {
	return shoot.Spec.DNS != nil && shoot.Spec.DNS.Domain != nil
}

func isDefaultDomain(domain string, defaultDomains []string) bool {
	for _, defaultDomain := range defaultDomains {
		if strings.HasSuffix(domain, "."+defaultDomain) {
			return true
		}
	}
	return false
}

func setPrimaryDNSProvider(shoot *core.Shoot, defaultDomains []string) error {
	dns := shoot.Spec.DNS
	if dns == nil {
		return nil
	}

	if dns.Domain != nil && isDefaultDomain(*dns.Domain, defaultDomains) {
		return nil
	}

	primary := helper.FindPrimaryDNSProvider(dns.Providers)
	if primary == nil && len(dns.Providers) > 0 {
		dns.Providers[0].Primary = ptr.To(true)
	}
	return nil
}

// assignDefaultDomainIfNeeded generates a domain <shoot-name>.<project-name>.<default-domain>
// and sets it in the shoot resource in the `spec.dns.domain` field.
// If for any reason no domain can be generated, no domain is assigned to the Shoot.
func assignDefaultDomainIfNeeded(shoot *core.Shoot, projectLister gardencorev1beta1listers.ProjectLister, defaultDomains []string) error {
	if shoot.Spec.DNS == nil {
		shoot.Spec.DNS = &core.DNS{}
	}

	if len(defaultDomains) > 0 && shoot.Spec.DNS.Domain == nil {
		domain := defaultDomains[0]
		shootDNSName := shoot.Name

		if len(shoot.Name) == 0 && len(shoot.GenerateName) > 0 {
			var err error
			shootDNSName, err = utils.GenerateRandomStringFromCharset(len(shoot.GenerateName)+5, "0123456789abcdefghijklmnopqrstuvwxyz")
			if err != nil {
				return apierrors.NewInternalError(err)
			}
		}

		project, err := admissionutils.ProjectForNamespaceFromLister(projectLister, shoot.Namespace)
		if err != nil {
			return apierrors.NewInternalError(err)
		}
		generatedDomain := fmt.Sprintf("%s.%s.%s", shootDNSName, project.Name, domain)
		shoot.Spec.DNS.Domain = &generatedDomain
	}

	return nil
}

func getDefaultDomains(secretLister kubecorev1listers.SecretLister, seedLister gardencorev1beta1listers.SeedLister, shoot *core.Shoot) ([]string, error) {
	var defaultDomains []string

	if shoot.Spec.SeedName != nil {
		seed, err := seedLister.Get(*shoot.Spec.SeedName)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("could not get seed %s: %v", *shoot.Spec.SeedName, err))
		}

		// If seed has DNS defaults configured, use those instead of the global defaults
		if len(seed.Spec.DNS.Defaults) > 0 {
			for _, seedDNSDefault := range seed.Spec.DNS.Defaults {
				defaultDomains = append(defaultDomains, seedDNSDefault.Domain)
			}
			return defaultDomains, nil
		}
	}

	selector, err := labels.Parse(fmt.Sprintf("%s=%s", v1beta1constants.GardenRole, v1beta1constants.GardenRoleDefaultDomain))
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}
	domainSecrets, err := secretLister.Secrets(v1beta1constants.GardenNamespace).List(selector)
	if err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	// sort domainSecrets with DNSDefaultDomainPriority to get the domain with the highest priority in first place
	sort.SliceStable(domainSecrets, func(i, j int) bool {
		iAnnotations := domainSecrets[i].GetAnnotations()
		jAnnotations := domainSecrets[j].GetAnnotations()
		var iPriority, jPriority int
		if iAnnotations != nil {
			if domainPriority, ok := iAnnotations[gardenerutils.DNSDefaultDomainPriority]; ok {
				iPriority, err = strconv.Atoi(domainPriority)
				if err != nil {
					iPriority = 0
				}
			}
		}
		if jAnnotations != nil {
			if domainPriority, ok := jAnnotations[gardenerutils.DNSDefaultDomainPriority]; ok {
				jPriority, err = strconv.Atoi(domainPriority)
				if err != nil {
					jPriority = 0
				}
			}
		}
		return iPriority > jPriority
	})

	for _, domainSecret := range domainSecrets {
		_, domain, _, err := gardenerutils.GetDomainInfoFromAnnotations(domainSecret.GetAnnotations())
		if err != nil {
			return nil, err
		}
		defaultDomains = append(defaultDomains, domain)
	}
	return defaultDomains, nil
}

// Validate validates the Shoot DNS specification.
func (d *DNS) Validate(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	if err := d.waitUntilReady(a); err != nil {
		return fmt.Errorf("err while waiting for ready: %w", err)
	}

	if shouldIgnore(a) {
		return nil
	}

	shoot, ok := a.GetObject().(*core.Shoot)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Shoot object")
	}

	defaultDomains, err := getDefaultDomains(d.secretLister, d.seedLister, shoot)
	if err != nil {
		return fmt.Errorf("error retrieving default domains: %s", err)
	}

	var allErrs field.ErrorList
	allErrs = append(allErrs, checkPrimaryDNSProvider(shoot, defaultDomains)...)

	switch a.GetOperation() {
	case admission.Create:
		// If shoot uses default domain, validate domain even though shoot can be assigned to seed
		// having dns disabled later on
		if isShootDomainSet(shoot) && !helper.ShootUsesUnmanagedDNS(shoot) {
			errs, err := checkDefaultDomainFormat(shoot, d.projectLister, defaultDomains)
			if err != nil {
				return err
			}

			allErrs = append(allErrs, errs...)
		}

	case admission.Update:
		oldShoot, ok := a.GetOldObject().(*core.Shoot)
		if !ok {
			return apierrors.NewBadRequest("could not convert old resource into Shoot object")
		}

		// Only validate domain on updates if the shoot's domain was not set previously and is being currently set.
		// This is necessary to avoid reconciliation errors for older shoots that already use a domain with an incorrect format.
		// There is also a possibility that an old shoot had an invalid domain, but was never assigned to a seed. This is why we check
		// if the shoot was previously not assigned to a seed and if the shoot's domain is invalid, the update is denied so that the invalid
		// domain does not get created.
		if (oldShoot.Spec.SeedName == nil || !isShootDomainSet(oldShoot)) && isShootDomainSet(shoot) && !helper.ShootUsesUnmanagedDNS(shoot) {
			errs, err := checkDefaultDomainFormat(shoot, d.projectLister, defaultDomains)
			if err != nil {
				return err
			}

			allErrs = append(allErrs, errs...)
		}
	}

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(a.GetKind().GroupKind(), shoot.Name, allErrs)
	}

	return nil
}

// checkPrimaryDNSProvider checks if the shoot uses a default domain and returns an error
// if a primary provider is used at the same time.
func checkPrimaryDNSProvider(shoot *core.Shoot, defaultDomains []string) field.ErrorList {
	dns := shoot.Spec.DNS
	if dns == nil || dns.Domain == nil || len(dns.Providers) == 0 {
		return nil
	}

	var allErrs field.ErrorList
	if isDefaultDomain(*dns.Domain, defaultDomains) {
		for i, provider := range dns.Providers {
			if ptr.Deref(provider.Primary, false) {
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "dns", "providers").Index(i).Child("primary"), provider.Primary, "primary dns provider must not be set when default domain is used"))
			}
		}
	}

	return allErrs
}

func checkDefaultDomainFormat(shoot *core.Shoot, projectLister gardencorev1beta1listers.ProjectLister, defaultDomains []string) (field.ErrorList, error) {
	var allErrs field.ErrorList

	project, err := admissionutils.ProjectForNamespaceFromLister(projectLister, shoot.Namespace)
	if err != nil {
		return allErrs, apierrors.NewInternalError(err)
	}

	shootDomain := shoot.Spec.DNS.Domain

	for _, domain := range defaultDomains {
		if strings.HasSuffix(*shootDomain, "."+domain) {
			// Check that the specified domain matches the pattern for default domains, especially in order
			// to prevent shoots from "stealing" domain names for shoots in other projects
			if len(shoot.GenerateName) > 0 && (len(shoot.Name) == 0 || strings.HasPrefix(shoot.Name, shoot.GenerateName)) {
				// Case where shoot name is generated or to be generated
				if !strings.HasSuffix(*shootDomain, fmt.Sprintf(".%s.%s", project.Name, domain)) {
					detail := fmt.Sprintf("shoot with 'metadata.generateName' uses a default domain but does not match expected scheme: <random-subdomain>.<project-name>.<default-domain> (expected '.%s.%s' to be a suffix of '%s')", project.Name, domain, *shootDomain)
					allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "dns", "domain"), shoot.Spec.DNS.Domain, detail))
					return allErrs, nil
				}
				return nil, nil
			}
			if *shootDomain != fmt.Sprintf("%s.%s.%s", shoot.Name, project.Name, domain) {
				detail := fmt.Sprintf("shoot uses a default domain but does not match expected scheme: <shoot-name>.<project-name>.<default-domain> (expected '%s.%s.%s', but got '%s')", shoot.Name, project.Name, domain, *shootDomain)
				allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "dns", "domain"), shoot.Spec.DNS.Domain, detail))
				return allErrs, nil
			}

			return nil, nil
		}
	}

	return nil, nil
}
