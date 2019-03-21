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
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gardener/gardener/pkg/apis/garden"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"

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
	projectLister gardenlisters.ProjectLister
	readyFunc     admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalGardenInformerFactory(&DNS{})
	_ = admissioninitializer.WantsKubeInformerFactory(&DNS{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new DNS admission plugin.
func New() (*DNS, error) {
	return &DNS{
		Handler: admission.NewHandler(admission.Create),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (d *DNS) AssignReadyFunc(f admission.ReadyFunc) {
	d.readyFunc = f
	d.SetReadyFunc(f)
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (d *DNS) SetInternalGardenInformerFactory(f gardeninformers.SharedInformerFactory) {
	projectInformer := f.Garden().InternalVersion().Projects()
	d.projectLister = projectInformer.Lister()

	readyFuncs = append(readyFuncs, projectInformer.Informer().HasSynced)
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
	return nil
}

// Admit tries to determine a DNS hosted zone for the Shoot's external domain.
func (d *DNS) Admit(a admission.Attributes, o admission.ObjectInterfaces) error {
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
	if a.GetKind().GroupKind() != garden.Kind("Shoot") {
		return nil
	}
	shoot, ok := a.GetObject().(*garden.Shoot)
	if !ok {
		return apierrors.NewBadRequest("could not convert resource into Shoot object")
	}

	// If the Shoot manifest specifies the 'unmanaged' DNS provider, then we do nothing.
	if provider := shoot.Spec.DNS.Provider; provider != nil && *provider == garden.DNSUnmanaged {
		return nil
	}

	// Generate a Shoot domain if none is configured.
	assignDefaultDomainIfNeeded(shoot, d.projectLister, d.secretLister)

	// If the provider != unmanaged then we need a configured domain.
	if shoot.Spec.DNS.Domain == nil {
		return apierrors.NewBadRequest(fmt.Sprintf("shoot domain field .spec.dns.domain must be set if provider != %s", garden.DNSUnmanaged))
	}

	return nil
}

// assignDefaultDomainIfNeeded generates a domain <shoot-name>.<project-name>.<default-domain>
// and sets it in the shoot resource in the `spec.dns.domain` field.
// If for any reason no domain can be generated, no domain is assigned to the Shoot.
func assignDefaultDomainIfNeeded(shoot *garden.Shoot, projectLister gardenlisters.ProjectLister, secretLister kubecorev1listers.SecretLister) {
	project, err := admissionutils.GetProject(shoot.Namespace, projectLister)
	if err != nil {
		return
	}

	selector, err := labels.Parse(fmt.Sprintf("%s=%s", common.GardenRole, common.GardenRoleDefaultDomain))
	if err != nil {
		return
	}
	secrets, err := secretLister.Secrets(common.GardenNamespace).List(selector)
	if err != nil {
		return
	}
	if len(secrets) == 0 {
		return
	}

	shootDomain := shoot.Spec.DNS.Domain

	for _, secret := range secrets {
		_, domain, err := common.GetDomainInfoFromAnnotations(secret.Annotations)
		if err != nil {
			return
		}

		if shootDomain != nil && strings.HasSuffix(*shootDomain, domain) {
			// Shoot already specifies a default domain, set provider to nil
			shoot.Spec.DNS.Provider = nil
			return
		}

		// Shoot did not specify a domain, assign default domain and set provider to nil
		if shootDomain == nil {
			generatedDomain := fmt.Sprintf("%s.%s.%s", shoot.Name, project.Name, domain)
			shoot.Spec.DNS.Provider = nil
			shoot.Spec.DNS.Domain = &generatedDomain
			return
		}
	}
}
