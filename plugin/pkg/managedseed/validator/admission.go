// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/helper"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	coreclientset "github.com/gardener/gardener/pkg/client/core/clientset/internalversion"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	corelisters "github.com/gardener/gardener/pkg/client/core/listers/core/internalversion"
	clientkubernetes "github.com/gardener/gardener/pkg/client/kubernetes"
	seedmanagementclientset "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ManagedSeedValidator"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ValidateManagedSeed contains listers and and admission handler.
type ValidateManagedSeed struct {
	*admission.Handler
	shootLister          corelisters.ShootLister
	secretBindingLister  corelisters.SecretBindingLister
	secretLister         kubecorev1listers.SecretLister
	coreClient           coreclientset.Interface
	seedManagementClient seedmanagementclientset.Interface
	readyFunc            admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsInternalCoreInformerFactory(&ValidateManagedSeed{})
	_ = admissioninitializer.WantsInternalCoreClientset(&ValidateManagedSeed{})
	_ = admissioninitializer.WantsSeedManagementClientset(&ValidateManagedSeed{})
	_ = admissioninitializer.WantsKubeInformerFactory(&ValidateManagedSeed{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new ValidateManagedSeed admission plugin.
func New() (*ValidateManagedSeed, error) {
	return &ValidateManagedSeed{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (v *ValidateManagedSeed) AssignReadyFunc(f admission.ReadyFunc) {
	v.readyFunc = f
	v.SetReadyFunc(f)
}

// SetInternalCoreInformerFactory gets Lister from SharedInformerFactory.
func (v *ValidateManagedSeed) SetInternalCoreInformerFactory(f coreinformers.SharedInformerFactory) {
	shootInformer := f.Core().InternalVersion().Shoots()
	v.shootLister = shootInformer.Lister()

	secretBindingInformer := f.Core().InternalVersion().SecretBindings()
	v.secretBindingLister = secretBindingInformer.Lister()

	readyFuncs = append(readyFuncs, shootInformer.Informer().HasSynced, secretBindingInformer.Informer().HasSynced)
}

// SetKubeInformerFactory gets Lister from SharedInformerFactory.
func (v *ValidateManagedSeed) SetKubeInformerFactory(f kubeinformers.SharedInformerFactory) {
	secretInformer := f.Core().V1().Secrets()
	v.secretLister = secretInformer.Lister()

	readyFuncs = append(readyFuncs, secretInformer.Informer().HasSynced)
}

// SetInternalCoreClientset sets the garden core clientset.
func (v *ValidateManagedSeed) SetInternalCoreClientset(c coreclientset.Interface) {
	v.coreClient = c
}

// SetSeedManagementClientset sets the garden seedmanagement clientset.
func (v *ValidateManagedSeed) SetSeedManagementClientset(c seedmanagementclientset.Interface) {
	v.seedManagementClient = c
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *ValidateManagedSeed) ValidateInitialization() error {
	if v.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	if v.secretBindingLister == nil {
		return errors.New("missing secret binding lister")
	}
	if v.secretLister == nil {
		return errors.New("missing secret lister")
	}
	if v.coreClient == nil {
		return errors.New("missing garden core client")
	}
	if v.seedManagementClient == nil {
		return errors.New("missing garden seedmanagement client")
	}
	return nil
}

var _ admission.MutationInterface = &ValidateManagedSeed{}

// Admit validates and if appropriate mutates the given managed seed against the shoot that it references.
func (v *ValidateManagedSeed) Admit(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
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

	// Ignore all kinds other than ManagedSeed
	if a.GetKind().GroupKind() != seedmanagement.Kind("ManagedSeed") {
		return nil
	}

	// Ignore updates to status or other subresources
	if a.GetSubresource() != "" {
		return nil
	}

	// Convert object to ManagedSeed
	managedSeed, ok := a.GetObject().(*seedmanagement.ManagedSeed)
	if !ok {
		return apierrors.NewBadRequest("could not convert object to ManagedSeed")
	}

	var allErrs field.ErrorList
	gk := schema.GroupKind{Group: seedmanagement.GroupName, Kind: "ManagedSeed"}

	// Ensure shoot name is specified
	shootNamePath := field.NewPath("spec", "shoot", "name")
	if managedSeed.Spec.Shoot.Name == "" {
		return apierrors.NewInvalid(gk, managedSeed.Name, append(allErrs, field.Required(shootNamePath, "shoot name is required")))
	}

	// Get shoot
	shoot, err := v.getShoot(ctx, managedSeed.Namespace, managedSeed.Spec.Shoot.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return apierrors.NewInvalid(gk, managedSeed.Name, append(allErrs, field.Invalid(shootNamePath, managedSeed.Spec.Shoot.Name, fmt.Sprintf("shoot %s/%s not found", managedSeed.Namespace, managedSeed.Spec.Shoot.Name))))
		}
		return apierrors.NewInternalError(fmt.Errorf("could not get shoot %s/%s: %v", managedSeed.Namespace, managedSeed.Spec.Shoot.Name, err))
	}

	// Ensure shoot can be registered as seed (specifies a domain)
	if shoot.Spec.DNS == nil || shoot.Spec.DNS.Domain == nil || *shoot.Spec.DNS.Domain == "" {
		return apierrors.NewInvalid(gk, managedSeed.Name, append(allErrs, field.Invalid(shootNamePath, managedSeed.Spec.Shoot.Name, fmt.Sprintf("shoot %s does not specify a domain", kutil.ObjectName(shoot)))))
	}

	// Ensure shoot is not already registered as seed
	ms, err := kutil.GetManagedSeed(ctx, v.seedManagementClient, managedSeed.Namespace, managedSeed.Spec.Shoot.Name)
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not get managed seed for shoot %s/%s: %v", managedSeed.Namespace, managedSeed.Spec.Shoot.Name, err))
	}
	if ms != nil && ms.Name != managedSeed.Name {
		return apierrors.NewInvalid(gk, managedSeed.Name, append(allErrs, field.Invalid(shootNamePath, managedSeed.Spec.Shoot.Name, fmt.Sprintf("shoot %s already registered as seed", kutil.ObjectName(shoot)))))
	}

	switch {
	case managedSeed.Spec.SeedTemplate != nil:
		// Admit seed spec against shoot
		errs, err := v.admitSeedSpec(ctx, &managedSeed.Spec.SeedTemplate.Spec, shoot, field.NewPath("spec", "seedTemplate", "spec"))
		if err != nil {
			return err
		}
		allErrs = append(allErrs, errs...)

	case managedSeed.Spec.Gardenlet != nil:
		// Admit gardelnet against shoot
		errs, err := v.admitGardenlet(ctx, managedSeed.Spec.Gardenlet, shoot, field.NewPath("spec", "gardenlet"))
		if err != nil {
			return err
		}
		allErrs = append(allErrs, errs...)
	}

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(gk, managedSeed.Name, allErrs)
	}

	return nil
}

func (v *ValidateManagedSeed) admitGardenlet(ctx context.Context, gardenlet *seedmanagement.Gardenlet, shoot *gardencore.Shoot, fldPath *field.Path) (field.ErrorList, error) {
	var allErrs field.ErrorList

	if gardenlet.Config != nil {
		configPath := fldPath.Child("config")

		// Decode gardenlet config to an internal version
		// Without defaults, since we don't want to set gardenlet config defaults in the resource at this point
		gardenletConfig, err := helper.ConvertGardenletConfigExternal(gardenlet.Config)
		if err != nil {
			return append(allErrs, field.Invalid(configPath, gardenlet.Config, fmt.Sprintf("could not decode config: %v", err))), nil
		}

		if gardenletConfig.SeedConfig != nil {
			seedConfigPath := fldPath.Child("seedConfig")

			// Convert seed config to internal version
			seed, err := helper.ConvertSeed(&gardenletConfig.SeedConfig.Seed)
			if err != nil {
				return allErrs, apierrors.NewInternalError(fmt.Errorf("could not convert seed config: %v", err))
			}

			// Admit seed spec against shoot
			errs, err := v.admitSeedSpec(ctx, &seed.Spec, shoot, seedConfigPath.Child("spec"))
			if err != nil {
				return allErrs, err
			}
			allErrs = append(allErrs, errs...)

			// Set seed config back to gardenletConfig.SeedConfig.Seed
			seedx, err := helper.ConvertSeedExternal(seed)
			if err != nil {
				return allErrs, apierrors.NewInternalError(fmt.Errorf("could not convert seed config: %v", err))
			}
			gardenletConfig.SeedConfig.Seed = *seedx
		}

		// Set gardenlet config back to gardenlet.Config
		gardenlet.Config = gardenletConfig
	}

	return allErrs, nil
}

func (v *ValidateManagedSeed) admitSeedSpec(ctx context.Context, spec *gardencore.SeedSpec, shoot *gardencore.Shoot, fldPath *field.Path) (field.ErrorList, error) {
	var allErrs field.ErrorList

	// Initialize backup provider
	if spec.Backup != nil && spec.Backup.Provider == "" {
		spec.Backup.Provider = shoot.Spec.Provider.Type
	}

	// Initialize and validate DNS and ingress
	ingressDomain := fmt.Sprintf("%s.%s", common.IngressPrefix, *(shoot.Spec.DNS.Domain))
	if spec.Ingress != nil {
		if gardencorehelper.NginxIngressEnabled(shoot.Spec.Addons) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("ingress"), fmt.Sprintf("seed ingress controller cannot be enabled on shoot %s", kutil.ObjectName(shoot))))
		}

		if spec.DNS.Provider == nil {
			dnsProvider, err := v.getSeedDNSProvider(ctx, shoot)
			if err != nil {
				if apierrors.IsInternalError(err) {
					return allErrs, err
				}
				allErrs = append(allErrs, field.Invalid(fldPath.Child("ingress"), spec.Ingress, err.Error()))
			}
			spec.DNS.Provider = dnsProvider
		}
		// TODO If spec.DNS.Provider is not nil should we check if the user specified exactly the DNS provider we found above?
		// I assume not, since the shoot may have multiple DNS providers

		if spec.Ingress.Domain == "" {
			spec.Ingress.Domain = ingressDomain
		} else if spec.Ingress.Domain != ingressDomain {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("ingress", "domain"), spec.Ingress.Domain, fmt.Sprintf("seed ingress domain must be equal to shoot DNS domain %s", ingressDomain)))
		}
	} else {
		if spec.DNS.IngressDomain == nil || *spec.DNS.IngressDomain == "" {
			spec.DNS.IngressDomain = &ingressDomain
		} else if *spec.DNS.IngressDomain != ingressDomain {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("dns", "ingressDomain"), spec.DNS.IngressDomain, fmt.Sprintf("seed ingress domain must be equal to shoot DNS domain %s", ingressDomain)))
		}
	}

	// Initialize and validate networks
	if spec.Networks.Nodes == nil {
		spec.Networks.Nodes = shoot.Spec.Networking.Nodes
	} else if shoot.Spec.Networking.Nodes != nil && *spec.Networks.Nodes != *shoot.Spec.Networking.Nodes {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networks", "nodes"), spec.Networks.Nodes, fmt.Sprintf("seed nodes CIDR must be equal to shoot nodes CIDR %s", *shoot.Spec.Networking.Nodes)))
	}
	if spec.Networks.Pods == "" && shoot.Spec.Networking.Pods != nil {
		spec.Networks.Pods = *shoot.Spec.Networking.Pods
	} else if shoot.Spec.Networking.Pods != nil && spec.Networks.Pods != *shoot.Spec.Networking.Pods {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networks", "pods"), spec.Networks.Pods, fmt.Sprintf("seed pods CIDR must be equal to shoot pods CIDR %s", *shoot.Spec.Networking.Pods)))
	}
	if spec.Networks.Services == "" && shoot.Spec.Networking.Services != nil {
		spec.Networks.Services = *shoot.Spec.Networking.Services
	} else if shoot.Spec.Networking.Services != nil && spec.Networks.Services != *shoot.Spec.Networking.Services {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networks", "services"), spec.Networks.Pods, fmt.Sprintf("seed services CIDR must be equal to shoot services CIDR %s", *shoot.Spec.Networking.Services)))
	}

	// Initialize and validate provider
	if spec.Provider.Type == "" {
		spec.Provider.Type = shoot.Spec.Provider.Type
	} else if spec.Provider.Type != shoot.Spec.Provider.Type {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("provider", "type"), spec.Provider.Type, fmt.Sprintf("seed provider type must be equal to shoot provider type %s", shoot.Spec.Provider.Type)))
	}
	if spec.Provider.Region == "" {
		spec.Provider.Region = shoot.Spec.Region
	} else if spec.Provider.Region != shoot.Spec.Region {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("provider", "region"), spec.Provider.Region, fmt.Sprintf("seed provider region must be equal to shoot region %s", shoot.Spec.Region)))
	}

	return allErrs, nil
}

func (v *ValidateManagedSeed) getSeedDNSProvider(ctx context.Context, shoot *gardencore.Shoot) (*gardencore.SeedDNSProvider, error) {
	dnsProvider, err := v.getSeedDNSProviderForCustomDomain(ctx, shoot)
	if err != nil {
		return nil, err
	}
	if dnsProvider == nil {
		dnsProvider, err = v.getSeedDNSProviderForDefaultDomain(ctx, shoot)
		if err != nil {
			return nil, err
		}
	}
	if dnsProvider == nil {
		return nil, fmt.Errorf("domain of shoot %s is neither a custom domain nor a default domain", kutil.ObjectName(shoot))
	}
	return dnsProvider, nil
}

func (v *ValidateManagedSeed) getSeedDNSProviderForCustomDomain(ctx context.Context, shoot *gardencore.Shoot) (*gardencore.SeedDNSProvider, error) {
	// Find a primary DNS provider in the list of shoot DNS providers
	primaryProvider := gardencorehelper.FindPrimaryDNSProvider(shoot.Spec.DNS.Providers)
	if primaryProvider == nil {
		return nil, nil
	}
	if primaryProvider.Type == nil {
		return nil, fmt.Errorf("primary DNS provider of shoot %s does not have a type", kutil.ObjectName(shoot))
	}
	if *primaryProvider.Type == gardencore.DNSUnmanaged {
		return nil, nil
	}

	// Initialize a reference to the primary DNS provider secret
	var secretRef corev1.SecretReference
	if primaryProvider.SecretName != nil {
		secretRef.Name = *primaryProvider.SecretName
		secretRef.Namespace = shoot.Namespace
	} else {
		secretBinding, err := v.getSecretBinding(shoot.Namespace, shoot.Spec.SecretBindingName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("secret binding %s/%s not found", shoot.Namespace, shoot.Spec.SecretBindingName)
			}
			return nil, apierrors.NewInternalError(fmt.Errorf("could not get secret binding %s/%s: %v", shoot.Namespace, shoot.Spec.SecretBindingName, err))
		}
		secretRef = secretBinding.SecretRef
	}

	return &gardencore.SeedDNSProvider{
		Type:      *primaryProvider.Type,
		SecretRef: secretRef,
		Domains:   primaryProvider.Domains,
		Zones:     primaryProvider.Zones,
	}, nil
}

func (v *ValidateManagedSeed) getSeedDNSProviderForDefaultDomain(ctx context.Context, shoot *gardencore.Shoot) (*gardencore.SeedDNSProvider, error) {
	// Get all default domain secrets in the garden namespace
	defaultDomainSecrets, err := v.getSecrets(v1beta1constants.GardenNamespace, labels.SelectorFromValidatedSet(map[string]string{
		v1beta1constants.GardenRole: common.GardenRoleDefaultDomain,
	}))
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("could not list default domain secrets in namespace %s: %v", v1beta1constants.GardenNamespace, err))
	}

	// Search for a default domain secret that matches the shoot domain
	for _, secret := range defaultDomainSecrets {
		provider, domain, includeZones, excludeZones, err := common.GetDomainInfoFromAnnotations(secret.Annotations)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("could not get domain info from domain secret annotations: %v", err))
		}

		if strings.HasSuffix(*shoot.Spec.DNS.Domain, domain) {
			var zones *gardencore.DNSIncludeExclude
			if includeZones != nil || excludeZones != nil {
				zones = &gardencore.DNSIncludeExclude{
					Include: includeZones,
					Exclude: excludeZones,
				}
			}

			return &gardencore.SeedDNSProvider{
				Type: provider,
				SecretRef: corev1.SecretReference{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
				Domains: &gardencore.DNSIncludeExclude{
					Include: []string{domain},
				},
				Zones: zones,
			}, nil
		}
	}

	return nil, nil
}

func (v *ValidateManagedSeed) getShoot(ctx context.Context, namespace, name string) (*gardencore.Shoot, error) {
	shoot, err := v.shootLister.Shoots(namespace).Get(name)
	if err != nil && apierrors.IsNotFound(err) {
		// Read from the client to ensure that if the managed seed has been created shortly after the shoot
		// and the shoot is not yet present in the lister cache, it could still be found
		return v.coreClient.Core().Shoots(namespace).Get(ctx, name, clientkubernetes.DefaultGetOptions())
	}
	return shoot, err
}

func (v *ValidateManagedSeed) getSecretBinding(namespace, name string) (*gardencore.SecretBinding, error) {
	return v.secretBindingLister.SecretBindings(namespace).Get(name)
}

func (v *ValidateManagedSeed) getSecrets(namespace string, selector labels.Selector) ([]*corev1.Secret, error) {
	return v.secretLister.Secrets(namespace).List(selector)
}
