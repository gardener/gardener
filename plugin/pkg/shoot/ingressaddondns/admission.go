// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package ingressaddondns

import (
	"errors"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/util/sets"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/gardener/gardener/pkg/apis/garden"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ShootIngressAddonDNS"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// IngressAddon contains listers and and admission handler.
type IngressAddon struct {
	*admission.Handler
	secretLister kubecorev1listers.SecretLister
	shootLister  gardenlisters.ShootLister
	readyFunc    admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalGardenInformerFactory(&IngressAddon{})
	_ = admissioninitializer.WantsKubeInformerFactory(&IngressAddon{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new IngressAddon admission plugin.
func New() (*IngressAddon, error) {
	return &IngressAddon{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (d *IngressAddon) AssignReadyFunc(f admission.ReadyFunc) {
	d.readyFunc = f
	d.SetReadyFunc(f)
}

// SetInternalGardenInformerFactory gets Lister from SharedInformerFactory.
func (d *IngressAddon) SetInternalGardenInformerFactory(f gardeninformers.SharedInformerFactory) {
	shootInformer := f.Garden().InternalVersion().Shoots()
	d.shootLister = shootInformer.Lister()

	readyFuncs = append(readyFuncs, shootInformer.Informer().HasSynced)
}

// SetKubeInformerFactory gets Lister from SharedInformerFactory.
func (d *IngressAddon) SetKubeInformerFactory(f kubeinformers.SharedInformerFactory) {
	secretInformer := f.Core().V1().Secrets()
	d.secretLister = secretInformer.Lister()

	readyFuncs = append(readyFuncs, secretInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (d *IngressAddon) ValidateInitialization() error {
	if d.secretLister == nil {
		return errors.New("missing secret lister")
	}
	if d.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	return nil
}

// Admit assures that internal Ingress records are not modified or set by users.
// This verification must happen first before a standard value can be calculated and added to the Shoot manifest.
// If this wasn't checked first, we wouldn't know whether these Ingress records had been generated or entered by users.
func (d *IngressAddon) Admit(a admission.Attributes) error {
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

	// Consider also disabled NginxIngress because it holds a state getting relevant when re-activating.
	addonDefined := shoot.Spec.Addons != nil && shoot.Spec.Addons.NginxIngress != nil

	if a.GetOperation() == admission.Create {
		// Prevent additionalRecords from being changed during creation.
		if addonDefined && len(shoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords) != 0 {
			return admission.NewForbidden(a, fmt.Errorf("additionalRecords is expected to be empty but found %v",
				shoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords))
		}
	}

	if a.GetOperation() == admission.Update {
		old, ok := a.GetOldObject().(*garden.Shoot)
		if !ok {
			return apierrors.NewInternalError(errors.New("could not convert old resource into Shoot object"))
		}

		// Prevent additionalRecords from being changed through update.
		if addonDefined && old.Spec.Addons != nil && old.Spec.Addons.NginxIngress != nil {
			if len(old.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords) !=
				len(shoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords) {
				return admission.NewForbidden(a, fmt.Errorf("additionalRecords %v must not be changed",
					shoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords))
			}

			oldRecords := sets.NewString(old.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords...)
			if !oldRecords.HasAll(shoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords...) {
				return admission.NewForbidden(a, fmt.Errorf("additionalRecords %v must not be changed",
					shoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords))
			}
		}

		// Prevent additionalRecords from being set if NginxIngress was not specified before.
		if addonDefined && (old.Spec.Addons == nil || old.Spec.Addons.NginxIngress == nil) {
			if len(shoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords) != 0 {
				return admission.NewForbidden(a, fmt.Errorf("additionalRecords is expected to be empty but found %v",
					shoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords))
			}
		}
	}

	// This case is only relevant during migration phase.
	// TODO: remove in future version.
	shootInDeletion := shoot.ObjectMeta.DeletionTimestamp != nil

	// Generate internal ingress domain if necessary.
	if addonDefined && !shootInDeletion && len(shoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords) == 0 {
		generatedDomain, err := d.generateAdditionalRecord()
		if err != nil {
			return err
		}
		shoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords = []string{generatedDomain}
	}

	// Generate default ingress domain if necessary.
	if addonDefined && !shootInDeletion && len(shoot.Spec.Addons.NginxIngress.IngressDNS.StandardRecords) == 0 &&
		shoot.Spec.DNS.Domain != nil && len(*shoot.Spec.DNS.Domain) > 0 {
		ingressDomain := utils.GenerateIngressDomain(*shoot.Spec.DNS.Domain)
		shoot.Spec.Addons.NginxIngress.IngressDNS.StandardRecords = []string{ingressDomain}
	}

	return nil
}

func (d *IngressAddon) generateAdditionalRecord() (string, error) {
	dnsSecret, err := admissionutils.GetInternalDomainSecret(d.secretLister)
	if err != nil {
		return "", apierrors.NewInternalError(err)
	}

	domain, ok := dnsSecret.Annotations[common.DNSDomain]
	if !ok {
		return "", apierrors.NewInternalError(fmt.Errorf("could not determine internal domain since annotation %s is not available", common.DNSDomain))
	}

	var (
		domainFound     bool
		generatedDomain string
	)

	// Try to find an unused internal domain but for not more than 10 rounds.
	for i := 0; !domainFound && i < 10; i++ {
		prefixLimit := 11
		prefix, err := utils.GenerateRandomStringFromCharset(prefixLimit, "0123456789abcdefghijklmnopqrstuvwxyz")
		if err != nil {
			return "", apierrors.NewInternalError(err)
		}

		generatedDomain = fmt.Sprintf("%s.%s", prefix, domain)
		shoots, err := d.shootLister.List(labels.Everything())
		if err != nil {
			return "", apierrors.NewInternalError(err)
		}

		var domainInUse bool
		for _, shoot := range shoots {
			domainInUse = false
			if shoot.Spec.Addons != nil && shoot.Spec.Addons.NginxIngress != nil {
				for _, domain := range shoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords {
					if domain == generatedDomain {
						domainInUse = true
						break
					}
				}
				if domainInUse {
					break
				}
			}
		}
		domainFound = !domainInUse
	}

	if !domainFound {
		return "", apierrors.NewInternalError(errors.New("could not calculate an additionalRecord for Shoot"))
	}
	return generatedDomain, nil
}
