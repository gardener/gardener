// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardenadmadmission

import (
	"context"
	"fmt"
	"io"

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

	"github.com/gardener/gardener/pkg/apis/core"
	_ "github.com/gardener/gardener/pkg/apis/core/install" // ensure internal types are registered for scheme conversion
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apiserver"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	shootregistry "github.com/gardener/gardener/pkg/apiserver/registry/core/shoot"
	gardencorefake "github.com/gardener/gardener/pkg/client/core/clientset/versioned/fake"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardensecurityfake "github.com/gardener/gardener/pkg/client/security/clientset/versioned/fake"
	securityinformers "github.com/gardener/gardener/pkg/client/security/informers/externalversions"
	fakeseedmanagement "github.com/gardener/gardener/pkg/client/seedmanagement/clientset/versioned/fake"
	seedmanagementinformers "github.com/gardener/gardener/pkg/client/seedmanagement/informers/externalversions"
	"github.com/gardener/gardener/pkg/gardenadm"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// RunAdmissionChain runs the gardener-apiserver admission chain (mutating plugins,
// static validation, and validating plugins) against the Shoot in resources.
// Mutations are applied in-memory: resources.Shoot is updated with the defaulted Shoot.
func RunAdmissionChain(ctx context.Context, log logr.Logger, resources *gardenadm.Resources) error {
	logger := log.WithValues("shoot", client.ObjectKeyFromObject(resources.Shoot))
	logger.Info("Running admission chain for Shoot")

	var (
		gardenerCoreClient    = gardencorefake.NewSimpleClientset(buildGardenerCoreObjects(resources)...)
		gardenerCoreInformers = gardencoreinformers.NewSharedInformerFactory(gardenerCoreClient, 0)

		gardenerSecurityClient    = gardensecurityfake.NewSimpleClientset(buildGardenerSecurityObjects(resources)...)
		gardenerSecurityInformers = securityinformers.NewSharedInformerFactory(gardenerSecurityClient, 0)

		gardenerSeedManagementClient    = fakeseedmanagement.NewSimpleClientset()
		gardenerSeedManagementInformers = seedmanagementinformers.NewSharedInformerFactory(gardenerSeedManagementClient, 0)

		kubeClient    = kubefake.NewSimpleClientset(buildKubeObjects(resources)...)
		kubeInformers = kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	)

	internalShoot := &core.Shoot{}
	if err := kubernetes.GardenScheme.Convert(resources.Shoot, internalShoot, nil); err != nil {
		return fmt.Errorf("failed converting Shoot to internal type: %w", err)
	}

	strategy := shootregistry.NewStrategy(0)
	strategy.PrepareForCreate(ctx, internalShoot)

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
				Name:   v1beta1constants.GardenadmUserNamePrefix + "init",
				Groups: []string{v1beta1constants.ShootsGroup},
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

	// Build the chain before starting informers: plugin initialization registers informers
	// with the factories, so Start/WaitForCacheSync only starts what the plugins need.
	chain, err := buildChain(initializer)
	if err != nil {
		return fmt.Errorf("failed initializing admission plugins: %w", err)
	}

	gardenerCoreInformers.Start(ctx.Done())
	gardenerCoreInformers.WaitForCacheSync(ctx.Done())
	gardenerSecurityInformers.Start(ctx.Done())
	gardenerSecurityInformers.WaitForCacheSync(ctx.Done())
	gardenerSeedManagementInformers.Start(ctx.Done())
	gardenerSeedManagementInformers.WaitForCacheSync(ctx.Done())
	kubeInformers.Start(ctx.Done())
	kubeInformers.WaitForCacheSync(ctx.Done())

	if err := chain.(admission.MutationInterface).Admit(ctx, attrs, objectInterfaces); err != nil {
		return fmt.Errorf("admission mutation rejected: %w", err)
	}

	if errs := strategy.Validate(ctx, internalShoot); len(errs) > 0 {
		return fmt.Errorf("static Shoot validation failed: %w", errs.ToAggregate())
	}

	strategy.Canonicalize(internalShoot)

	if err := chain.(admission.ValidationInterface).Validate(ctx, attrs, objectInterfaces); err != nil {
		return fmt.Errorf("admission validation rejected: %w", err)
	}

	resources.Shoot = &gardencorev1beta1.Shoot{}
	if err := kubernetes.GardenScheme.Convert(internalShoot, resources.Shoot, nil); err != nil {
		return fmt.Errorf("failed converting mutated Shoot back to v1beta1: %w", err)
	}

	kubernetes.GardenScheme.Default(resources.Shoot)

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

// buildChain creates an admission.Interface chain from all applicable default-on admission plugins.
// Only default-on plugins not in GardenadmSkipPlugins are included. New default-on plugins are
// picked up automatically — add to GardenadmSkipPlugins only if explicitly inapplicable in gardenadm.
func buildChain(initializer admission.PluginInitializer) (admission.Interface, error) {
	registry := admission.NewPlugins()
	apiserver.RegisterAllAdmissionPlugins(registry)

	var (
		defaultOnPlugins = plugin.DefaultOnPlugins()
		skippedPlugins   = plugin.GardenadmSkipPlugins()
		names            []string
	)

	for _, name := range plugin.AllOrderedPluginNames() {
		if defaultOnPlugins.Has(name) && !skippedPlugins.Has(name) {
			names = append(names, name)
		}
	}

	decorator := admission.DecoratorFunc(func(handler admission.Interface, _ string) admission.Interface {
		if assignable, ok := handler.(interface{ AssignReadyFunc(admission.ReadyFunc) }); ok {
			assignable.AssignReadyFunc(func() bool { return true })
		}
		return handler
	})

	return registry.NewFromPlugins(names, noopConfigProvider{}, initializer, decorator)
}

// noopConfigProvider implements admission.ConfigProvider and returns nil config for every plugin.
type noopConfigProvider struct{}

func (noopConfigProvider) ConfigFor(_ string) (io.Reader, error) { return nil, nil }
