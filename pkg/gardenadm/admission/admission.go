// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardenadmadmission

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/quota/v1/generic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corevalidation "github.com/gardener/gardener/pkg/api/core/validation"
	"github.com/gardener/gardener/pkg/apis/core"
	_ "github.com/gardener/gardener/pkg/apis/core/install" // ensure internal types are registered for scheme conversion
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apiserver"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencorefake "github.com/gardener/gardener/pkg/client/core/clientset/versioned/fake"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenSecurityFake "github.com/gardener/gardener/pkg/client/security/clientset/versioned/fake"
	securityinformers "github.com/gardener/gardener/pkg/client/security/informers/externalversions"
	fakeseedmanagement "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned/fake"
	seedmanagementinformers "github.com/gardener/gardener/pkg/client/seedmanagement/informers/externalversions"
	"github.com/gardener/gardener/pkg/gardenadm"
	"github.com/gardener/gardener/pkg/utils/flow"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// RunAdmissionChain runs the gardener-apiserver admission chain (mutating plugins,
// static validation, and validating plugins) against the Shoot in resources.
// Mutations are applied in-memory: resources.Shoot is updated with the defaulted Shoot.
func RunAdmissionChain(ctx context.Context, log logr.Logger, resources *gardenadm.Resources) error {
	logger := log.WithValues("shoot", client.ObjectKeyFromObject(resources.Shoot))
	logger.Info("Running admission chain for Shoot")

	var (
		gardenerCoreObjects   = buildGardenerCoreObjects(resources)
		gardenerCoreClient    = gardencorefake.NewSimpleClientset(gardenerCoreObjects...)
		gardenerCoreInformers = gardencoreinformers.NewSharedInformerFactory(nil, 0)

		gardenerSecurityObjects   = buildGardenerSecurityObjects(resources)
		gardenerSecurityClient    = gardenSecurityFake.NewSimpleClientset(gardenerSecurityObjects...)
		gardenerSecurityInformers = securityinformers.NewSharedInformerFactory(nil, 0)

		gardenerSeedManagementClient    = fakeseedmanagement.NewSimpleClientset()
		gardenerSeedManagementInformers = seedmanagementinformers.NewSharedInformerFactory(nil, 0)

		kubeObjects   = buildKubeObjects(resources)
		kubeClient    = kubefake.NewSimpleClientset(kubeObjects...)
		kubeInformers = kubeinformers.NewSharedInformerFactory(nil, 0)
	)

	if err := populateInformerStores(resources, gardenerCoreInformers, gardenerSecurityInformers, kubeInformers); err != nil {
		return fmt.Errorf("failed seeding informer stores: %w", err)
	}

	internalShoot := &core.Shoot{}
	if err := kubernetes.GardenScheme.Convert(resources.Shoot, internalShoot, nil); err != nil {
		return fmt.Errorf("failed converting Shoot to internal type: %w", err)
	}

	var (
		attrs = admission.NewAttributesRecord(
			internalShoot,
			nil, // CREATE — no old object
			core.Kind("Shoot").WithVersion("v1beta1"),
			internalShoot.Namespace,
			internalShoot.Name,
			core.Resource("shoots").WithVersion("v1beta1"),
			"",
			admission.Create,
			&metav1.CreateOptions{},
			false,
			&user.DefaultInfo{
				Name:   "gardener.cloud:gardenadm:shoot:init",
				Groups: []string{"gardener.cloud:system:shoots"},
			},
		)

		initializer = admissioninitializer.New(
			gardenerCoreInformers,
			gardenerCoreClient,
			gardenerSeedManagementInformers,
			gardenerSeedManagementClient,
			gardenerSecurityInformers,
			gardenerSecurityClient,
			kubeInformers,
			kubeClient,
			dynamicfake.NewSimpleDynamicClient(kubernetes.GardenScheme),
			authorizer.AuthorizerFunc(func(_ context.Context, _ authorizer.Attributes) (authorizer.Decision, string, error) {
				return authorizer.DecisionAllow, "", nil
			}),
			generic.NewConfiguration(nil, nil),
		)

		objectInterfaces = admission.NewObjectInterfacesFromScheme(kubernetes.GardenScheme)
	)

	plugins, err := buildPlugins(initializer)
	if err != nil {
		return fmt.Errorf("failed initializing admission plugins: %w", err)
	}

	for _, plugin := range plugins {
		mutator, ok := plugin.(admission.MutationInterface)
		if !ok {
			continue
		}
		if err := mutator.Admit(ctx, attrs, objectInterfaces); err != nil {
			return fmt.Errorf("admission plugin %T rejected mutation: %w", plugin, err)
		}
	}

	if errs := corevalidation.ValidateShoot(internalShoot); len(errs) > 0 {
		return fmt.Errorf("static Shoot validation failed: %w", errs.ToAggregate())
	}

	var validateTaskFns []flow.TaskFn
	for _, plugin := range plugins {
		if validator, ok := plugin.(admission.ValidationInterface); ok {
			validateTaskFns = append(validateTaskFns, func(ctx context.Context) error {
				if err := validator.Validate(ctx, attrs, objectInterfaces); err != nil {
					return fmt.Errorf("admission plugin %T rejected validation: %w", plugin, err)
				}
				return nil
			})
		}
	}
	if err := flow.Parallel(validateTaskFns...)(ctx); err != nil {
		return err
	}

	resources.Shoot = &gardencorev1beta1.Shoot{}
	if err := kubernetes.GardenScheme.Convert(internalShoot, resources.Shoot, nil); err != nil {
		return fmt.Errorf("failed converting mutated Shoot back to v1beta1: %w", err)
	}

	logger.Info("Admission chain completed successfully")
	return nil
}

// buildGardenerCoreObjects constructs the list of runtime.Object to seed into the fake core clientset.
func buildGardenerCoreObjects(resources *gardenadm.Resources) []runtime.Object {
	objects := []runtime.Object{
		resources.CloudProfile,
		resources.Project,
		resources.Shoot,
	}
	for _, obj := range resources.ControllerRegistrations {
		objects = append(objects, obj)
	}
	for _, obj := range resources.ControllerDeployments {
		objects = append(objects, obj)
	}
	if resources.SecretBinding != nil {
		objects = append(objects, resources.SecretBinding)
	}
	return objects
}

// buildGardenerSecurityObjects constructs the list of runtime.Object to seed into the fake security clientset.
func buildGardenerSecurityObjects(resources *gardenadm.Resources) []runtime.Object {
	var objects []runtime.Object
	if resources.CredentialsBinding != nil {
		objects = append(objects, resources.CredentialsBinding)
	}
	for _, obj := range resources.WorkloadIdentities {
		objects = append(objects, obj)
	}
	return objects
}

// buildKubeObjects constructs the list of runtime.Object to seed into the fake kube clientset.
func buildKubeObjects(resources *gardenadm.Resources) []runtime.Object {
	var objects []runtime.Object
	for _, obj := range resources.Secrets {
		objects = append(objects, obj)
	}
	for _, obj := range resources.ConfigMaps {
		objects = append(objects, obj)
	}
	return objects
}

// populateInformerStores seeds the informer stores with the resources.
func populateInformerStores(
	resources *gardenadm.Resources,
	gardenerCoreInformers gardencoreinformers.SharedInformerFactory,
	gardenerSecurityInformers securityinformers.SharedInformerFactory,
	kubeInformers kubeinformers.SharedInformerFactory,
) error {
	if err := gardenerCoreInformers.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(resources.CloudProfile); err != nil {
		return fmt.Errorf("failed adding CloudProfile to store: %w", err)
	}
	if err := gardenerCoreInformers.Core().V1beta1().Projects().Informer().GetStore().Add(resources.Project); err != nil {
		return fmt.Errorf("failed adding Project to store: %w", err)
	}
	if err := gardenerCoreInformers.Core().V1beta1().Shoots().Informer().GetStore().Add(resources.Shoot); err != nil {
		return fmt.Errorf("failed adding Shoot to store: %w", err)
	}
	for _, obj := range resources.ControllerRegistrations {
		if err := gardenerCoreInformers.Core().V1beta1().ControllerRegistrations().Informer().GetStore().Add(obj); err != nil {
			return fmt.Errorf("failed adding ControllerRegistration to store: %w", err)
		}
	}
	for _, obj := range resources.ControllerDeployments {
		if err := gardenerCoreInformers.Core().V1().ControllerDeployments().Informer().GetStore().Add(obj); err != nil {
			return fmt.Errorf("failed adding ControllerDeployment to store: %w", err)
		}
	}
	if resources.SecretBinding != nil {
		if err := gardenerCoreInformers.Core().V1beta1().SecretBindings().Informer().GetStore().Add(resources.SecretBinding); err != nil {
			return fmt.Errorf("failed adding SecretBinding to store: %w", err)
		}
	}

	if resources.CredentialsBinding != nil {
		if err := gardenerSecurityInformers.Security().V1alpha1().CredentialsBindings().Informer().GetStore().Add(resources.CredentialsBinding); err != nil {
			return fmt.Errorf("failed adding CredentialsBinding to store: %w", err)
		}
	}
	for _, obj := range resources.WorkloadIdentities {
		if err := gardenerSecurityInformers.Security().V1alpha1().WorkloadIdentities().Informer().GetStore().Add(obj); err != nil {
			return fmt.Errorf("failed adding WorkloadIdentity to store: %w", err)
		}
	}

	for _, obj := range resources.Secrets {
		if err := kubeInformers.Core().V1().Secrets().Informer().GetStore().Add(obj); err != nil {
			return fmt.Errorf("failed adding Secret to store: %w", err)
		}
	}
	for _, obj := range resources.ConfigMaps {
		if err := kubeInformers.Core().V1().ConfigMaps().Informer().GetStore().Add(obj); err != nil {
			return fmt.Errorf("failed adding ConfigMap to store: %w", err)
		}
	}

	return nil
}

// buildPlugins creates and initializes all admission plugins relevant for gardenadm in order.
// Only default-on plugins are considered; plugins that are not applicable in the gardenadm
// context must be added to GardenadmSkipPlugins.
func buildPlugins(initializer admission.PluginInitializer) ([]admission.Interface, error) {
	registry := admission.NewPlugins()
	apiserver.RegisterAllAdmissionPlugins(registry)

	var (
		defaultOnPlugins = plugin.DefaultOnPlugins()
		skippedPlugins   = plugin.GardenadmSkipPlugins()
		plugins          []admission.Interface
	)

	for _, pluginName := range plugin.AllOrderedPluginNames() {
		if !defaultOnPlugins.Has(pluginName) || skippedPlugins.Has(pluginName) {
			continue
		}

		p, err := registry.InitPlugin(pluginName, nil, initializer)
		if err != nil {
			return nil, fmt.Errorf("failed initializing admission plugin %q: %w", pluginName, err)
		}
		if p == nil {
			continue
		}

		if assignable, ok := p.(interface{ AssignReadyFunc(admission.ReadyFunc) }); ok {
			assignable.AssignReadyFunc(func() bool { return true })
		}
		plugins = append(plugins, p)
	}

	return plugins, nil
}
