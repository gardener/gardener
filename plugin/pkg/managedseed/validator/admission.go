// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	seedmanagementhelper "github.com/gardener/gardener/pkg/apis/seedmanagement/helper"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	securityinformers "github.com/gardener/gardener/pkg/client/security/informers/externalversions"
	securityv1alpha1listers "github.com/gardener/gardener/pkg/client/security/listers/security/v1alpha1"
	seedmanagementclientset "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	plugin "github.com/gardener/gardener/plugin/pkg"
	admissionutils "github.com/gardener/gardener/plugin/pkg/utils"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameManagedSeed, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// ManagedSeed contains listers and admission handler.
type ManagedSeed struct {
	*admission.Handler
	shootLister              gardencorev1beta1listers.ShootLister
	secretBindingLister      gardencorev1beta1listers.SecretBindingLister
	credentialsBindingLister securityv1alpha1listers.CredentialsBindingLister
	secretLister             kubecorev1listers.SecretLister
	coreClient               gardencoreclientset.Interface
	seedManagementClient     seedmanagementclientset.Interface
	readyFunc                admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreInformerFactory(&ManagedSeed{})
	_ = admissioninitializer.WantsCoreClientSet(&ManagedSeed{})
	_ = admissioninitializer.WantsSeedManagementClientSet(&ManagedSeed{})
	_ = admissioninitializer.WantsKubeInformerFactory(&ManagedSeed{})
	_ = admissioninitializer.WantsSecurityInformerFactory(&ManagedSeed{})

	readyFuncs []admission.ReadyFunc
)

// New creates a new ManagedSeed admission plugin.
func New() (*ManagedSeed, error) {
	return &ManagedSeed{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (v *ManagedSeed) AssignReadyFunc(f admission.ReadyFunc) {
	v.readyFunc = f
	v.SetReadyFunc(f)
}

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (v *ManagedSeed) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	shootInformer := f.Core().V1beta1().Shoots()
	v.shootLister = shootInformer.Lister()

	secretBindingInformer := f.Core().V1beta1().SecretBindings()
	v.secretBindingLister = secretBindingInformer.Lister()

	readyFuncs = append(readyFuncs, shootInformer.Informer().HasSynced, secretBindingInformer.Informer().HasSynced)
}

// SetSecurityInformerFactory gets Lister from SharedInformerFactory.
func (v *ManagedSeed) SetSecurityInformerFactory(f securityinformers.SharedInformerFactory) {
	credentialsBindingInformer := f.Security().V1alpha1().CredentialsBindings()
	v.credentialsBindingLister = credentialsBindingInformer.Lister()

	readyFuncs = append(readyFuncs, credentialsBindingInformer.Informer().HasSynced)
}

// SetKubeInformerFactory gets Lister from SharedInformerFactory.
func (v *ManagedSeed) SetKubeInformerFactory(f kubeinformers.SharedInformerFactory) {
	secretInformer := f.Core().V1().Secrets()
	v.secretLister = secretInformer.Lister()

	readyFuncs = append(readyFuncs, secretInformer.Informer().HasSynced)
}

// SetCoreClientSet sets the garden core clientset.
func (v *ManagedSeed) SetCoreClientSet(c gardencoreclientset.Interface) {
	v.coreClient = c
}

// SetSeedManagementClientSet sets the garden seedmanagement clientset.
func (v *ManagedSeed) SetSeedManagementClientSet(c seedmanagementclientset.Interface) {
	v.seedManagementClient = c
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (v *ManagedSeed) ValidateInitialization() error {
	if v.shootLister == nil {
		return errors.New("missing shoot lister")
	}
	if v.secretBindingLister == nil {
		return errors.New("missing secret binding lister")
	}
	if v.credentialsBindingLister == nil {
		return errors.New("missing credentials binding lister")
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

var _ admission.MutationInterface = &ManagedSeed{}

// Admit validates and if appropriate mutates the given managed seed against the shoot that it references.
func (v *ManagedSeed) Admit(ctx context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
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

	// Ensure namespace is garden
	// Garden namespace validation can be disabled by disabling the ManagedSeed plugin for integration test.
	if managedSeed.Namespace != v1beta1constants.GardenNamespace {
		return apierrors.NewInvalid(gk, managedSeed.Name, append(allErrs, field.Invalid(field.NewPath("metadata", "namespace"), managedSeed.Namespace, "namespace must be garden")))
	}

	// Ensure shoot and shoot name are specified
	shootPath := field.NewPath("spec", "shoot")
	shootNamePath := shootPath.Child("name")

	if managedSeed.Spec.Shoot == nil {
		return apierrors.NewInvalid(gk, managedSeed.Name, append(allErrs, field.Required(shootPath, "shoot is required")))
	}
	if managedSeed.Spec.Shoot.Name == "" {
		return apierrors.NewInvalid(gk, managedSeed.Name, append(allErrs, field.Required(shootNamePath, "shoot name is required")))
	}

	shoot, err := v.getShoot(ctx, managedSeed.Namespace, managedSeed.Spec.Shoot.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return apierrors.NewInvalid(gk, managedSeed.Name, append(allErrs, field.Invalid(shootNamePath, managedSeed.Spec.Shoot.Name, fmt.Sprintf("shoot %s/%s not found", managedSeed.Namespace, managedSeed.Spec.Shoot.Name))))
		}
		return apierrors.NewInternalError(fmt.Errorf("could not get shoot %s/%s: %v", managedSeed.Namespace, managedSeed.Spec.Shoot.Name, err))
	}

	// Ensure shoot can be registered as seed
	if shoot.Spec.DNS == nil || shoot.Spec.DNS.Domain == nil || *shoot.Spec.DNS.Domain == "" {
		return apierrors.NewInvalid(gk, managedSeed.Name, append(allErrs, field.Invalid(shootNamePath, managedSeed.Spec.Shoot.Name, fmt.Sprintf("shoot %s does not specify a domain", client.ObjectKeyFromObject(shoot)))))
	}
	if v1beta1helper.NginxIngressEnabled(shoot.Spec.Addons) {
		return apierrors.NewInvalid(gk, managedSeed.Name, append(allErrs, field.Invalid(shootNamePath, managedSeed.Spec.Shoot.Name, "shoot ingress addon is not supported for managed seeds - use the managed seed ingress controller")))
	}
	if !v1beta1helper.ShootWantsVerticalPodAutoscaler(shoot) {
		return apierrors.NewInvalid(gk, managedSeed.Name, append(allErrs, field.Invalid(shootNamePath, managedSeed.Spec.Shoot.Name, "shoot VPA has to be enabled for managed seeds")))
	}
	if v1beta1helper.IsWorkerless(shoot) {
		return apierrors.NewInvalid(gk, managedSeed.Name, append(allErrs, field.Invalid(shootNamePath, managedSeed.Spec.Shoot.Name, "workerless shoot cannot be used to create managed seed")))
	}

	// Ensure shoot is not already registered as seed
	ms, err := admissionutils.GetManagedSeed(ctx, v.seedManagementClient, managedSeed.Namespace, managedSeed.Spec.Shoot.Name)
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not get managed seed for shoot %s/%s: %v", managedSeed.Namespace, managedSeed.Spec.Shoot.Name, err))
	}
	if ms != nil && ms.Name != managedSeed.Name {
		return apierrors.NewInvalid(gk, managedSeed.Name, append(allErrs, field.Invalid(shootNamePath, managedSeed.Spec.Shoot.Name, fmt.Sprintf("shoot %s already registered as seed by managed seed %s", client.ObjectKeyFromObject(shoot), client.ObjectKeyFromObject(ms)))))
	}

	// Admit gardenlet against shoot
	errs, err := v.admitGardenlet(&managedSeed.Spec.Gardenlet, shoot, field.NewPath("spec", "gardenlet"))
	if err != nil {
		return err
	}
	allErrs = append(allErrs, errs...)

	gardenerutils.MaintainSeedNameLabels(managedSeed, shoot.Spec.SeedName, &managedSeed.Name)

	switch a.GetOperation() {
	case admission.Create:
		errs, err := v.validateManagedSeedCreate(managedSeed, shoot)
		if err != nil {
			return err
		}
		allErrs = append(allErrs, errs...)
	case admission.Update:
		oldManagedSeed, ok := a.GetOldObject().(*seedmanagement.ManagedSeed)
		if !ok {
			return apierrors.NewInternalError(errors.New("could not convert old resource into ManagedSeed object"))
		}
		errs, err := v.validateManagedSeedUpdate(oldManagedSeed, managedSeed, shoot)
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

func (v *ManagedSeed) validateManagedSeedCreate(managedSeed *seedmanagement.ManagedSeed, shoot *gardencorev1beta1.Shoot) (field.ErrorList, error) {
	allErrs := field.ErrorList{}

	seedSpec, err := seedmanagementhelper.ExtractSeedSpec(managedSeed)
	if err != nil {
		return nil, err
	}

	shootZones := v1beta1helper.GetAllZonesFromShoot(shoot)

	if !shootZones.HasAll(seedSpec.Provider.Zones...) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "gardenlet", "config", "seedConfig", "spec", "provider", "zones"), seedSpec.Provider.Zones, "cannot use zone in seed provider that is not available in referenced shoot"))
	}

	return allErrs, nil
}

func (v *ManagedSeed) validateManagedSeedUpdate(oldManagedSeed, newManagedSeed *seedmanagement.ManagedSeed, shoot *gardencorev1beta1.Shoot) (field.ErrorList, error) {
	allErrs := field.ErrorList{}
	zonesFieldPath := field.NewPath("spec", "gardenlet", "config", "seedConfig", "spec", "provider", "zones")

	oldSeedSpec, err := seedmanagementhelper.ExtractSeedSpec(oldManagedSeed)
	if err != nil {
		return nil, err
	}
	newSeedSpec, err := seedmanagementhelper.ExtractSeedSpec(newManagedSeed)
	if err != nil {
		return nil, err
	}

	if err := admissionutils.ValidateZoneRemovalFromSeeds(oldSeedSpec, newSeedSpec, newManagedSeed.Name, v.shootLister, "ManagedSeed"); err != nil {
		allErrs = append(allErrs, field.Forbidden(zonesFieldPath, "zones must not be removed while shoots are still scheduled onto seed"))
	}

	shootZones := v1beta1helper.GetAllZonesFromShoot(shoot)
	newZones := sets.New(newSeedSpec.Provider.Zones...).Difference(sets.New(oldSeedSpec.Provider.Zones...))

	// Newly added zones should match the ones found in the shoot cluster.
	// Zone names were allowed to deviate from the zones configured for shoot clusters, see https://github.com/gardener/gardener/commit/8d28452e7f718d0041fbe82eb83543e3a87ea8ad.
	// Thus, we can only check added zones here.
	if !shootZones.HasAll(newZones.UnsortedList()...) {
		allErrs = append(allErrs, field.Invalid(zonesFieldPath, newZones.UnsortedList(), "added zones must match zone names configured for workers in the referenced shoot cluster"))
	}

	return allErrs, nil
}

func (v *ManagedSeed) admitGardenlet(gardenlet *seedmanagement.GardenletConfig, shoot *gardencorev1beta1.Shoot, fldPath *field.Path) (field.ErrorList, error) {
	var allErrs field.ErrorList

	if gardenlet.Config != nil {
		configPath := fldPath.Child("config")

		gardenletConfig, ok := gardenlet.Config.(*gardenletconfigv1alpha1.GardenletConfiguration)
		if !ok {
			return allErrs, apierrors.NewInternalError(fmt.Errorf("expected *gardenletconfigv1alpha1.GardenletConfiguration but got %T", gardenlet.Config))
		}

		if gardenletConfig.SeedConfig != nil {
			seedConfigPath := configPath.Child("seedConfig")

			// Convert seed template config to an internal version
			seedTemplate, err := gardencorehelper.ConvertSeedTemplate(&gardenletConfig.SeedConfig.SeedTemplate)
			if err != nil {
				return allErrs, apierrors.NewInternalError(fmt.Errorf("could not convert seed template to internal version: %v", err))
			}

			// Admit seed spec against shoot
			errs, err := v.admitSeedSpec(&seedTemplate.Spec, shoot, seedConfigPath.Child("spec"))
			if err != nil {
				return allErrs, err
			}
			allErrs = append(allErrs, errs...)

			// Convert seed template to an external version and set it back to gardenletConfig.SeedConfig
			seedTemplateExternal, err := gardencorehelper.ConvertSeedTemplateExternal(seedTemplate)
			if err != nil {
				return allErrs, apierrors.NewInternalError(fmt.Errorf("could not convert seed template to external version: %v", err))
			}

			gardenletConfig.SeedConfig.SeedTemplate = *seedTemplateExternal
		}
	}

	return allErrs, nil
}

func (v *ManagedSeed) admitSeedSpec(spec *gardencore.SeedSpec, shoot *gardencorev1beta1.Shoot, fldPath *field.Path) (field.ErrorList, error) {
	var allErrs field.ErrorList

	// Initialize backup provider
	if spec.Backup != nil && spec.Backup.Provider == "" {
		spec.Backup.Provider = shoot.Spec.Provider.Type
	}

	// Initialize and validate DNS and ingress
	if spec.Ingress == nil {
		spec.Ingress = &gardencore.Ingress{
			Controller: gardencore.IngressController{
				Kind: v1beta1constants.IngressKindNginx,
			},
		}
	}

	if spec.DNS.Provider == nil {
		dnsProvider, err := v.getSeedDNSProvider(shoot)
		if err != nil {
			if apierrors.IsInternalError(err) {
				return allErrs, err
			}
			allErrs = append(allErrs, field.Invalid(fldPath.Child("ingress"), spec.Ingress, err.Error()))
		}
		spec.DNS.Provider = dnsProvider
	}

	ingressDomain := fmt.Sprintf("%s.%s", gardenerutils.IngressPrefix, *(shoot.Spec.DNS.Domain))
	if spec.Ingress.Domain == "" {
		spec.Ingress.Domain = ingressDomain
	} else if spec.Ingress.Domain != ingressDomain {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("ingress", "domain"), spec.Ingress.Domain, "seed ingress domain must be equal to shoot DNS domain "+ingressDomain))
	}

	// Initialize and validate networks
	if spec.Networks.Nodes == nil {
		spec.Networks.Nodes = shoot.Spec.Networking.Nodes
	} else if shoot.Spec.Networking.Nodes != nil && *spec.Networks.Nodes != *shoot.Spec.Networking.Nodes {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networks", "nodes"), spec.Networks.Nodes, "seed nodes CIDR must be equal to shoot nodes CIDR "+*shoot.Spec.Networking.Nodes))
	}
	if spec.Networks.Pods == "" && shoot.Spec.Networking.Pods != nil {
		spec.Networks.Pods = *shoot.Spec.Networking.Pods
	} else if shoot.Spec.Networking.Pods != nil && spec.Networks.Pods != *shoot.Spec.Networking.Pods {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networks", "pods"), spec.Networks.Pods, "seed pods CIDR must be equal to shoot pods CIDR "+*shoot.Spec.Networking.Pods))
	}
	if spec.Networks.Services == "" && shoot.Spec.Networking.Services != nil {
		spec.Networks.Services = *shoot.Spec.Networking.Services
	} else if shoot.Spec.Networking.Services != nil && spec.Networks.Services != *shoot.Spec.Networking.Services {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networks", "services"), spec.Networks.Pods, "seed services CIDR must be equal to shoot services CIDR "+*shoot.Spec.Networking.Services))
	}

	// Initialize and validate provider
	if spec.Provider.Type == "" {
		spec.Provider.Type = shoot.Spec.Provider.Type
	} else if spec.Provider.Type != shoot.Spec.Provider.Type {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("provider", "type"), spec.Provider.Type, "seed provider type must be equal to shoot provider type "+shoot.Spec.Provider.Type))
	}
	if spec.Provider.Region == "" {
		spec.Provider.Region = shoot.Spec.Region
	} else if spec.Provider.Region != shoot.Spec.Region {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("provider", "region"), spec.Provider.Region, "seed provider region must be equal to shoot region "+shoot.Spec.Region))
	}
	if shootZones := v1beta1helper.GetAllZonesFromShoot(shoot); len(spec.Provider.Zones) == 0 && shootZones.Len() > 0 {
		spec.Provider.Zones = sets.List(shootZones)
	}

	// At this point the Shoot VPA should be already enabled (validated earlier). If the Seed does not specify VPA settings,
	// disable the Seed VPA. If the Seed VPA is enabled, fail the validation.
	if spec.Settings == nil || spec.Settings.VerticalPodAutoscaler == nil {
		if spec.Settings == nil {
			spec.Settings = &gardencore.SeedSettings{}
		}
		spec.Settings.VerticalPodAutoscaler = &gardencore.SeedSettingVerticalPodAutoscaler{
			Enabled: false,
		}
	} else if spec.Settings.VerticalPodAutoscaler.Enabled {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("settings", "verticalPodAutoscaler", "enabled"), spec.Settings.VerticalPodAutoscaler.Enabled, "seed VPA is not supported for managed seeds - use the shoot VPA"))
	}

	topologyAwareRoutingEnabled := gardencorehelper.SeedSettingTopologyAwareRoutingEnabled(spec.Settings)
	if topologyAwareRoutingEnabled {
		if v1beta1helper.KubeAPIServerFeatureGateDisabled(shoot, "TopologyAwareHints") {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("settings", "topologyAwareRouting", "enabled"), spec.Settings.TopologyAwareRouting.Enabled, "the topology-aware routing seed setting cannot be enabled when the TopologyAwareHints feature gate is disabled for kube-apiserver"))
		}
		if v1beta1helper.KubeControllerManagerFeatureGateDisabled(shoot, "TopologyAwareHints") {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("settings", "topologyAwareRouting", "enabled"), spec.Settings.TopologyAwareRouting.Enabled, "the topology-aware routing seed setting cannot be enabled when the TopologyAwareHints feature gate is disabled for kube-controller-manager"))
		}
		if v1beta1helper.KubeProxyFeatureGateDisabled(shoot, "TopologyAwareHints") {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("settings", "topologyAwareRouting", "enabled"), spec.Settings.TopologyAwareRouting.Enabled, "the topology-aware routing seed setting cannot be enabled when the TopologyAwareHints feature gate is disabled for kube-proxy"))
		}
	}

	return allErrs, nil
}

func (v *ManagedSeed) getSeedDNSProvider(shoot *gardencorev1beta1.Shoot) (*gardencore.SeedDNSProvider, error) {
	dnsProvider, err := v.getSeedDNSProviderForCustomDomain(shoot)
	if err != nil {
		return nil, err
	}
	if dnsProvider == nil {
		dnsProvider, err = v.getSeedDNSProviderForDefaultDomain(shoot)
		if err != nil {
			return nil, err
		}
	}
	if dnsProvider == nil {
		return nil, fmt.Errorf("domain of shoot %s is neither a custom domain nor a default domain", client.ObjectKeyFromObject(shoot))
	}
	return dnsProvider, nil
}

func (v *ManagedSeed) getSeedDNSProviderForCustomDomain(shoot *gardencorev1beta1.Shoot) (*gardencore.SeedDNSProvider, error) {
	// Find a primary DNS provider in the list of shoot DNS providers
	primaryProvider := v1beta1helper.FindPrimaryDNSProvider(shoot.Spec.DNS.Providers)
	if primaryProvider == nil {
		return nil, nil
	}
	if primaryProvider.Type == nil {
		return nil, fmt.Errorf("primary DNS provider of shoot %s does not have a type", client.ObjectKeyFromObject(shoot))
	}
	if *primaryProvider.Type == gardencore.DNSUnmanaged {
		return nil, nil
	}

	// Initialize a reference to the primary DNS provider secret
	var secretRef corev1.SecretReference
	if primaryProvider.SecretName != nil {
		secretRef.Name = *primaryProvider.SecretName
		secretRef.Namespace = shoot.Namespace
	} else if shoot.Spec.SecretBindingName != nil {
		secretBinding, err := v.secretBindingLister.SecretBindings(shoot.Namespace).Get(*shoot.Spec.SecretBindingName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("secret binding %s/%s not found", shoot.Namespace, *shoot.Spec.SecretBindingName)
			}
			return nil, apierrors.NewInternalError(fmt.Errorf("could not get secret binding %s/%s: %v", shoot.Namespace, *shoot.Spec.SecretBindingName, err))
		}
		secretRef = secretBinding.SecretRef
	} else if shoot.Spec.CredentialsBindingName != nil {
		// TODO(dimityrmirchev): This code should eventually handle references to workload identity
		credentialsBinding, err := v.credentialsBindingLister.CredentialsBindings(shoot.Namespace).Get(*shoot.Spec.CredentialsBindingName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("credentials binding %s/%s not found", shoot.Namespace, *shoot.Spec.CredentialsBindingName)
			}
			return nil, apierrors.NewInternalError(fmt.Errorf("could not get secret binding %s/%s: %v", shoot.Namespace, *shoot.Spec.SecretBindingName, err))
		}
		secretRef = corev1.SecretReference{
			Name:      credentialsBinding.CredentialsRef.Name,
			Namespace: credentialsBinding.CredentialsRef.Namespace,
		}
	} else {
		return nil, fmt.Errorf("cannot initialize a reference to the primary DNS provider secret of shoot %s", client.ObjectKeyFromObject(shoot))
	}

	return &gardencore.SeedDNSProvider{
		Type:      *primaryProvider.Type,
		SecretRef: secretRef,
	}, nil
}

func (v *ManagedSeed) getSeedDNSProviderForDefaultDomain(shoot *gardencorev1beta1.Shoot) (*gardencore.SeedDNSProvider, error) {
	// Get all default domain secrets in the garden namespace
	defaultDomainSecrets, err := v.secretLister.Secrets(v1beta1constants.GardenNamespace).List(labels.SelectorFromValidatedSet(map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleDefaultDomain,
	}))
	if err != nil {
		return nil, apierrors.NewInternalError(fmt.Errorf("could not list default domain secrets in namespace %s: %v", v1beta1constants.GardenNamespace, err))
	}

	// Search for a default domain secret that matches the shoot domain
	for _, secret := range defaultDomainSecrets {
		provider, domain, _, err := gardenerutils.GetDomainInfoFromAnnotations(secret.Annotations)
		if err != nil {
			return nil, apierrors.NewInternalError(fmt.Errorf("could not get domain info from domain secret annotations: %v", err))
		}

		if strings.HasSuffix(*shoot.Spec.DNS.Domain, domain) {
			return &gardencore.SeedDNSProvider{
				Type: provider,
				SecretRef: corev1.SecretReference{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			}, nil
		}
	}

	return nil, nil
}

func (v *ManagedSeed) getShoot(ctx context.Context, namespace, name string) (*gardencorev1beta1.Shoot, error) {
	shoot, err := v.shootLister.Shoots(namespace).Get(name)
	if err != nil && apierrors.IsNotFound(err) {
		// Read from the client to ensure that if the managed seed has been created shortly after the shoot
		// and the shoot is not yet present in the lister cache, it could still be found
		shoot, err = v.coreClient.CoreV1beta1().Shoots(namespace).Get(ctx, name, kubernetesclient.DefaultGetOptions())
	}

	return shoot, err
}
