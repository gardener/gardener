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
	"strings"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	corelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
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
	shoot, ok := a.GetObject().(*core.Shoot)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Shoot object")
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

		if shoot.Spec.SeedName == nil {
			return nil
		}
		if oldShoot.Spec.SeedName != nil {
			return nil
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

	// If the Shoot manifest specifies the 'unmanaged' DNS provider, then we do nothing.
	if helper.ShootUsesUnmanagedDNS(shoot) {
		return nil
	}

	// Generate a Shoot domain if none is configured (at this point in time we know that the chosen seed does
	// not disable DNS.
	if err := assignDefaultDomainIfNeeded(shoot, d.projectLister, d.secretLister); err != nil {
		return err
	}

	// If the seed does not disable DNS and the shoot does not use the unmanaged DNS provider then we need
	// a configured domain.
	if shoot.Spec.DNS == nil || shoot.Spec.DNS.Domain == nil {
		return apierrors.NewBadRequest(fmt.Sprintf("shoot domain field .spec.dns.domain must be set if provider != %s and assigned to a seed which does not disable DNS", core.DNSUnmanaged))
	}
	return nil
}

func seedDisablesDNS(seedLister corelisters.SeedLister, seedName string) (bool, error) {
	seed, err := seedLister.Get(seedName)
	if err != nil {
		return false, err
	}
	return helper.TaintsHave(seed.Spec.Taints, core.SeedTaintDisableDNS), nil
}

// assignDefaultDomainIfNeeded generates a domain <shoot-name>.<project-name>.<default-domain>
// and sets it in the shoot resource in the `spec.dns.domain` field.
// If for any reason no domain can be generated, no domain is assigned to the Shoot.
func assignDefaultDomainIfNeeded(shoot *core.Shoot, projectLister corelisters.ProjectLister, secretLister kubecorev1listers.SecretLister) error {
	project, err := admissionutils.GetProject(shoot.Namespace, projectLister)
	if err != nil {
		return apierrors.NewInternalError(err)
	}

	var domainSecrets []*corev1.Secret
	deprecatedSelector, err := labels.Parse(fmt.Sprintf("%s=%s", v1beta1constants.DeprecatedGardenRole, common.GardenRoleDefaultDomain))
	if err != nil {
		return apierrors.NewInternalError(err)
	}
	secrets, err := secretLister.Secrets(v1beta1constants.GardenNamespace).List(deprecatedSelector)
	if err != nil {
		return apierrors.NewInternalError(err)
	}
	domainSecrets = append(domainSecrets, secrets...)

	selector, err := labels.Parse(fmt.Sprintf("%s=%s", v1beta1constants.GardenRole, common.GardenRoleDefaultDomain))
	if err != nil {
		return apierrors.NewInternalError(err)
	}
	secrets, err = secretLister.Secrets(v1beta1constants.GardenNamespace).List(selector)
	if err != nil {
		return apierrors.NewInternalError(err)
	}
	domainSecrets = append(domainSecrets, secrets...)

	if len(domainSecrets) == 0 {
		return nil
	}

	var shootDomain *string
	if shoot.Spec.DNS != nil {
		shootDomain = shoot.Spec.DNS.Domain
	}

	for _, secret := range domainSecrets {
		_, domain, _, _, err := common.GetDomainInfoFromAnnotations(secret.Annotations)
		if err != nil {
			return apierrors.NewInternalError(err)
		}

		if shootDomain != nil && strings.HasSuffix(*shootDomain, "."+domain) {
			// Shoot already specifies a default domain, set providers to nil
			shoot.Spec.DNS.Providers = nil

			// Check that the specified domain matches the pattern for default domains, especially in order
			// to prevent shoots from "stealing" domain names for shoots in other projects
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
			generatedDomain := fmt.Sprintf("%s.%s.%s", shoot.Name, project.Name, domain)
			shoot.Spec.DNS.Providers = nil
			shoot.Spec.DNS.Domain = &generatedDomain
			return nil
		}
	}

	return nil
}
