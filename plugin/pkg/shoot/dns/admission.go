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

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	corelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/utils/pointer"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ShootDNS"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// DNS contains listers and and admission handler.
type DNS struct {
	*admission.Handler
	secretLister  kubecorev1listers.SecretLister
	projectLister corelisters.ProjectLister
	seedLister    corelisters.SeedLister
	readyFunc     admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalCoreInformerFactory(&DNS{})
	_ = admissioninitializer.WantsKubeInformerFactory(&DNS{})

	readyFuncs = []admission.ReadyFunc{}
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

// SetInternalCoreInformerFactory gets Lister from SharedInformerFactory.
func (d *DNS) SetInternalCoreInformerFactory(f coreinformers.SharedInformerFactory) {
	projectInformer := f.Core().InternalVersion().Projects()
	d.projectLister = projectInformer.Lister()

	seedInformer := f.Core().InternalVersion().Seeds()
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

var _ admission.MutationInterface = &DNS{}

// Admit tries to determine a DNS hosted zone for the Shoot's external domain.
func (d *DNS) Admit(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
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

	// Ignore all kinds other than Shoot
	if a.GetKind().GroupKind() != core.Kind("Shoot") {
		return nil
	}
	// Ignore updates to shoot status or other subresources
	if a.GetSubresource() != "" {
		return nil
	}
	shoot, ok := a.GetObject().(*core.Shoot)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Shoot object")
	}

	defaultDomains, err := getDefaultDomains(d.secretLister)
	if err != nil {
		return fmt.Errorf("error retrieving default domains: %s", err)
	}

	if err := checkPrimaryDNSProvider(shoot.Spec.DNS, defaultDomains); err != nil {
		return err
	}

	// If a shoot is newly created and not yet assigned to a seed we do nothing. We need to know the seed
	// in order to check whether it's tainted to not use DNS.
	switch a.GetOperation() {
	case admission.Create:
		if shoot.Spec.SeedName == nil {
			return nil
		}

	case admission.Update:
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
					if reflect.DeepEqual(provider.Type, oldPrimaryProvider.Type) && reflect.DeepEqual(provider.SecretName, oldPrimaryProvider.SecretName) {
						shoot.Spec.DNS.Providers[i].Primary = pointer.Bool(true)
						break
					}
				}
			}
		}

		if shoot.Spec.SeedName == nil {
			return nil
		}

		if oldShoot.Spec.SeedName != nil {
			if *oldShoot.Spec.SeedName != *shoot.Spec.SeedName {
				if err := checkIfShootMigrationIsPossible(d.seedLister, oldShoot, shoot); err != nil {
					return err
				}
				if shoot.Spec.DNS != nil {
					return checkFunctionlessDNSProviders(shoot.Spec.DNS)
				}
				return nil
			}
			if shoot.Spec.DNS != nil {
				return checkFunctionlessDNSProviders(shoot.Spec.DNS)
			}
		}
	}

	dnsDisabled, err := seedDisablesDNS(d.seedLister, *shoot.Spec.SeedName)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("could not get referenced seed: %+v", err.Error()))
	}
	if dnsDisabled {
		if shoot.Spec.DNS != nil {
			return apierrors.NewBadRequest("shoot's .spec.dns section must be nil if seed with disabled DNS is chosen")
		}
		return nil
	}

	// Generate a Shoot domain if none is configured (at this point in time we know that the chosen seed does
	// not disable DNS.
	if !helper.ShootUsesUnmanagedDNS(shoot) {
		if err := assignDefaultDomainIfNeeded(shoot, d.projectLister, defaultDomains); err != nil {
			return err
		}

		if shoot.Spec.DNS == nil || shoot.Spec.DNS.Domain == nil {
			return apierrors.NewBadRequest(fmt.Sprintf("shoot domain field .spec.dns.domain must be set if provider != %s and assigned to a seed which does not disable DNS", core.DNSUnmanaged))
		}
	}

	if shoot.Spec.DNS != nil {
		if err := setPrimaryDNSProvider(shoot.Spec.DNS, defaultDomains); err != nil {
			return err
		}
		if err := checkFunctionlessDNSProviders(shoot.Spec.DNS); err != nil {
			return err
		}
	}
	return nil
}

// checkFunctionlessDNSProviders returns an error if a non-primary provider isn't configured correctly.
func checkFunctionlessDNSProviders(dns *core.DNS) error {
	for _, provider := range dns.Providers {
		if !utils.IsTrue(provider.Primary) && (provider.Type == nil || provider.SecretName == nil) {
			return apierrors.NewBadRequest("non-primary DNS providers in .spec.dns.providers must specify a `type` and `secretName`")
		}
	}
	return nil
}

func checkIfShootMigrationIsPossible(seedLister corelisters.SeedLister, oldShoot, newShoot *core.Shoot) error {
	for _, seedName := range []string{*oldShoot.Spec.SeedName, *newShoot.Spec.SeedName} {
		seedDNSDisabled, err := seedDisablesDNS(seedLister, seedName)
		if err != nil {
			return apierrors.NewBadRequest(fmt.Sprintf("could not get referenced seed: %+v", err.Error()))
		}
		if seedDNSDisabled {
			return apierrors.NewBadRequest("source and destination seeds must enable DNS so that the shoot can be migrated")
		}
	}
	return nil
}

// checkPrimaryDNSProvider checks if the shoot uses a default domain and returns an error
// if a primary provider is used at the same time.
func checkPrimaryDNSProvider(dns *core.DNS, defaultDomains []string) error {
	if dns == nil || dns.Domain == nil || len(dns.Providers) == 0 {
		return nil
	}

	var defaultDomain = isDefaultDomain(*dns.Domain, defaultDomains)
	if defaultDomain {
		primary := helper.FindPrimaryDNSProvider(dns.Providers)
		if primary != nil {
			return apierrors.NewBadRequest("primary dns provider must not be set when default domain is used")
		}
	}
	return nil
}

func isDefaultDomain(domain string, defaultDomains []string) bool {
	for _, defaultDomain := range defaultDomains {
		if strings.HasSuffix(domain, "."+defaultDomain) {
			return true
		}
	}
	return false
}

func setPrimaryDNSProvider(dns *core.DNS, defaultDomains []string) error {
	if dns == nil {
		return nil
	}
	if err := checkPrimaryDNSProvider(dns, defaultDomains); err != nil {
		return err
	}

	if dns.Domain != nil && isDefaultDomain(*dns.Domain, defaultDomains) {
		return nil
	}

	primary := helper.FindPrimaryDNSProvider(dns.Providers)
	if primary == nil && len(dns.Providers) > 0 {
		dns.Providers[0].Primary = pointer.Bool(true)
	}
	return nil
}

func seedDisablesDNS(seedLister corelisters.SeedLister, seedName string) (bool, error) {
	seed, err := seedLister.Get(seedName)
	if err != nil {
		return false, err
	}
	return !seed.Spec.Settings.ShootDNS.Enabled, nil
}

// assignDefaultDomainIfNeeded generates a domain <shoot-name>.<project-name>.<default-domain>
// and sets it in the shoot resource in the `spec.dns.domain` field.
// If for any reason no domain can be generated, no domain is assigned to the Shoot.
func assignDefaultDomainIfNeeded(shoot *core.Shoot, projectLister corelisters.ProjectLister, defaultDomains []string) error {
	project, err := gutil.ProjectForNamespaceFromInternalLister(projectLister, shoot.Namespace)
	if err != nil {
		return apierrors.NewInternalError(err)
	}

	var shootDomain *string
	if shoot.Spec.DNS != nil {
		shootDomain = shoot.Spec.DNS.Domain
	}

	for _, domain := range defaultDomains {
		if shootDomain != nil && strings.HasSuffix(*shootDomain, "."+domain) {
			// Check that the specified domain matches the pattern for default domains, especially in order
			// to prevent shoots from "stealing" domain names for shoots in other projects
			if len(shoot.GenerateName) > 0 && (len(shoot.Name) == 0 || strings.HasPrefix(shoot.Name, shoot.GenerateName)) {
				// Case where shoot name is generated or to be generated
				if !strings.HasSuffix(*shootDomain, fmt.Sprintf(".%s.%s", project.Name, domain)) {
					return apierrors.NewBadRequest("shoot with 'metadata.generateName' uses a default domain but does not match expected scheme: <random-subdomain>.<project-name>.<default-domain>")
				}
				return nil
			}
			if *shootDomain != fmt.Sprintf("%s.%s.%s", shoot.Name, project.Name, domain) {
				return apierrors.NewBadRequest("shoot uses a default domain but does not match expected scheme: <shoot-name>.<project-name>.<default-domain>")
			}

			return nil
		}

		// Shoot did not specify a domain, assign default domain and set provider to nil
		if shootDomain == nil {
			if shoot.Spec.DNS == nil {
				shoot.Spec.DNS = &core.DNS{}
			}
			shootDNSName := shoot.Name
			if len(shoot.Name) == 0 && len(shoot.GenerateName) > 0 {
				shootDNSName, err = utils.GenerateRandomStringFromCharset(len(shoot.GenerateName)+5, "0123456789abcdefghijklmnopqrstuvwxyz")
				if err != nil {
					return apierrors.NewInternalError(err)
				}
			}
			generatedDomain := fmt.Sprintf("%s.%s.%s", shootDNSName, project.Name, domain)
			shoot.Spec.DNS.Domain = &generatedDomain
			return nil
		}
	}

	return nil
}

func getDefaultDomains(secretLister kubecorev1listers.SecretLister) ([]string, error) {
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
			if domainPriority, ok := iAnnotations[gutil.DNSDefaultDomainPriority]; ok {
				iPriority, err = strconv.Atoi(domainPriority)
				if err != nil {
					iPriority = 0
				}
			}
		}
		if jAnnotations != nil {
			if domainPriority, ok := jAnnotations[gutil.DNSDefaultDomainPriority]; ok {
				jPriority, err = strconv.Atoi(domainPriority)
				if err != nil {
					jPriority = 0
				}
			}
		}
		return iPriority > jPriority
	})

	var defaultDomains []string
	for _, domainSecret := range domainSecrets {
		_, domain, _, _, _, err := gutil.GetDomainInfoFromAnnotations(domainSecret.GetAnnotations())
		if err != nil {
			return nil, err
		}
		defaultDomains = append(defaultDomains, domain)
	}
	return defaultDomains, nil
}
