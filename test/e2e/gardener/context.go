// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	corev1 "k8s.io/api/core/v1"
	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
)

// TestContext carries test state and helpers through e2e test cases with multiple steps (ordered containers).
// A TestContext value is supposed to be static, i.e., it should not be augmented with test case-specific members like
// a Shoot object. For this, more specific types should be derived like ShootContext.
// Hence, a single TestContext value can be reused by multiple test cases.
//
// The context types should not use ginkgo for initialization or cleanup, i.e., they should not automatically register
// BeforeEach or similar nodes. Instead, test cases must initialize the context explicitly during tree construction.
// This prevents incomprehensible test code.
//
// The context types should not offer methods for common test steps. Instead, common test steps should be implemented in
// dedicated helper functions accepting a context param. Attaching test steps (e.g., create and wait for shoot) to the
// context type would bloat the context type. Using dedicated helper functions for this, allows separating them by
// topic, e.g., seed-related and shoot-related test steps.
type TestContext struct {
	Log logr.Logger

	// GardenClientSet is a client for the garden cluster derived from the KUBECONFIG env var.
	GardenClientSet kubernetes.Interface
	// GardenClient is the controller-runtime client of the GardenClientSet. This is a more convenient equivalent of
	// GardenClientSet.Client().
	GardenClient client.Client
	// GardenKomega is a Komega instance for writing assertions on objects in the garden cluster. E.g.,
	//  Eventually(ctx, s.GardenKomega.Object(s.Shoot)).Should(
	//    HaveField("Status.LastOperation.State", Equal(gardencorev1beta1.LastOperationStateFailed)),
	//  )
	GardenKomega komega.Komega
}

// NewTestContext sets up a new TestContext for working with the garden cluster pointed to by the KUBECONFIG env var.
// As NewTestContext is expected to be called during tree construction, we cannot perform gomega assertions and have to
// handle errors by panicking.
func NewTestContext() *TestContext {
	t := &TestContext{
		Log: logger.MustNewZapLogger(logger.DebugLevel, logger.FormatText, logzap.WriteTo(GinkgoWriter)),
	}

	gardenScheme := kubernetes.GardenScheme
	utilruntime.Must(operatorv1alpha1.AddToScheme(gardenScheme))
	utilruntime.Must(resourcesv1alpha1.AddToScheme(gardenScheme))
	utilruntime.Must(apiextensionsscheme.AddToScheme(gardenScheme))

	gardenClientSet, err := kubernetes.NewClientFromFile("", os.Getenv("KUBECONFIG"),
		kubernetes.WithClientOptions(client.Options{Scheme: gardenScheme}),
		kubernetes.WithClientConnectionOptions(componentbaseconfigv1alpha1.ClientConnectionConfiguration{QPS: 100, Burst: 130}),
		kubernetes.WithAllowedUserFields([]string{kubernetes.AuthTokenFile}),
		kubernetes.WithDisabledCachedClient(),
	)
	if err != nil {
		panic(fmt.Errorf("failed to create garden client set: %w", err))
	}

	t.GardenClientSet = gardenClientSet
	t.GardenClient = gardenClientSet.Client()
	t.GardenKomega = komega.New(t.GardenClient)

	return t
}

// ForShoot copies the receiver TestContext for deriving a ShootContext.
func (t *TestContext) ForShoot(shoot *gardencorev1beta1.Shoot) *ShootContext {
	s := &ShootContext{
		TestContext: *t,
		Shoot:       shoot,
	}
	s.Log = s.Log.WithValues("shoot", client.ObjectKeyFromObject(shoot))

	return s
}

// ShootContext is a test case-specific TestContext that carries test state and helpers through multiple steps of the
// same test case, i.e., within the same ordered container.
// Accordingly, ShootContext values must not be reused across multiple test cases (ordered containers). Make sure to
// declare ShootContext variables within the ordered container and initialize them during ginkgo tree construction,
// e.g., in a BeforeTestSetup node or when invoking a shared `test` func.
//
// A ShootContext can be initialized using TestContext.ForShoot.
type ShootContext struct {
	TestContext

	// Shoot object that the test case is working with.
	Shoot *gardencorev1beta1.Shoot

	// ShootClientSet is a client for the shoot cluster. It must be initialized via WithShootClientSet.
	ShootClientSet kubernetes.Interface
	// ShootClient is the controller-runtime client of the ShootClientSet. This is a more convenient equivalent of
	// ShootClientSet.Client().
	ShootClient client.Client
	// ShootKomega is a Komega instance for writing assertions on objects in the shoot cluster. E.g.,
	//  Eventually(ctx, s.ShootKomega.ObjectList(&corev1.NodeList{})).Should(
	//    HaveField("Items", HaveLen(1)),
	//  )
	ShootKomega komega.Komega

	// Seed is the responsible Seed of the shoot.
	Seed *gardencorev1beta1.Seed

	// SeedClientSet is a client for the seed cluster. It must be initialized via WithSeedClientSet.
	SeedClientSet kubernetes.Interface
	// SeedClient is the controller-runtime client of the SeedClientSet. This is a more convenient equivalent of
	// SeedClientSet.Client().
	SeedClient client.Client
	// SeedKomega is a Komega instance for writing assertions on objects in the seed cluster. E.g.,
	//  Eventually(ctx, s.SeedKomega.ObjectList(&corev1.NodeList{})).Should(
	//    HaveField("Items", HaveLen(1)),
	//  )
	SeedKomega komega.Komega
}

// WithShootClientSet initializes the shoot clients of this ShootContext from the given client set.
func (s *ShootContext) WithShootClientSet(clientSet kubernetes.Interface) *ShootContext {
	s.ShootClientSet = clientSet
	s.ShootClient = clientSet.Client()
	s.ShootKomega = komega.New(s.ShootClient)
	return s
}

// WithSeedClientSet initializes the seed clients of this ShootContext from the given client set.
func (s *ShootContext) WithSeedClientSet(clientSet kubernetes.Interface) *ShootContext {
	s.SeedClientSet = clientSet
	s.SeedClient = clientSet.Client()
	s.SeedKomega = komega.New(s.SeedClient)
	return s
}

// ProjectContext is a test case-specific TestContext that carries test state and helpers through multiple steps of the
// same test case, i.e., within the same ordered container.
// Accordingly, ProjectContext values must not be reused across multiple test cases (ordered containers). Make sure to
// declare ProjectContext variables within the ordered container and initialize them during ginkgo tree construction,
// e.g., in a BeforeTestSetup node or when invoking a shared `test` func.
//
// A ProjectContext can be initialized using TestContext.ForProject.
type ProjectContext struct {
	TestContext
	Project *gardencorev1beta1.Project
}

// ForProject copies the receiver TestContext for deriving a ProjectContext.
func (t *TestContext) ForProject(project *gardencorev1beta1.Project) *ProjectContext {
	s := &ProjectContext{
		TestContext: *t,
		Project:     project,
	}
	s.Log = s.Log.WithValues("project", client.ObjectKeyFromObject(project))

	return s
}

// GardenContext is a test case-specific TestContext that carries test state and helpers through multiple steps of the
// same test case, i.e., within the same ordered container.
// Accordingly, GardenContext values must not be reused across multiple test cases (ordered containers). Make sure to
// declare GardenContext variables within the ordered container and initialize them during ginkgo tree construction,
// e.g., in a BeforeTestSetup node or when invoking a shared `test` func.
//
// A GardenContext can be initialized using TestContext.ForGarden.
type GardenContext struct {
	TestContext

	// Garden object the test is working with
	Garden *operatorv1alpha1.Garden

	// BackupSecret contains the backup secret the test is working with
	BackupSecret *corev1.Secret

	// VirtualClusterClientSet is a client for the virtual cluster. It must be initialized via WithVirtualClusterClientSet.
	VirtualClusterClientSet kubernetes.Interface
	// VirtualClusterClient is the controller-runtime client of the VirtualClusterClientSet. This is a more convenient equivalent of
	// VirtualClusterClientSet.Client().
	VirtualClusterClient client.Client
	// VirtualClusterKomega is a Komega instance for writing assertions on objects in the virtual cluster. E.g.,
	//  Eventually(ctx, s.VirtualClusterKomega.ObjectList(&corev1.NodeList{})).Should(
	//    HaveField("Items", HaveLen(1)),
	//  )
	VirtualClusterKomega komega.Komega
}

// ForGarden copies the receiver TestContext for deriving a GardenContext.
func (t *TestContext) ForGarden(garden *operatorv1alpha1.Garden, backupSecret *corev1.Secret) *GardenContext {
	s := &GardenContext{
		TestContext:  *t,
		Garden:       garden,
		BackupSecret: backupSecret,
	}
	s.Log = s.Log.WithValues("garden", client.ObjectKeyFromObject(garden))

	return s
}

// WithVirtualClusterClientSet initializes the virtual cluster clients of this GardenContext from the given client set.
func (s *GardenContext) WithVirtualClusterClientSet(clientSet kubernetes.Interface) *GardenContext {
	s.VirtualClusterClientSet = clientSet
	s.VirtualClusterClient = clientSet.Client()
	s.VirtualClusterKomega = komega.New(s.VirtualClusterClient)
	return s
}

// SeedContext is a test case-specific TestContext that carries test state and helpers through multiple steps of the
// same test case, i.e., within the same ordered container.
// Accordingly, SeedContext values must not be reused across multiple test cases (ordered containers). Make sure to
// declare SeedContext variables within the ordered container and initialize them during ginkgo tree construction,
// e.g., in a BeforeTestSetup node or when invoking a shared `test` func.
//
// A SeedContext can be initialized using TestContext.ForSeed.
type SeedContext struct {
	TestContext

	// Seed object the test is working with
	Seed *gardencorev1beta1.Seed
}

// ForSeed copies the receiver TestContext for deriving a SeedContext.
func (t *TestContext) ForSeed(seed *gardencorev1beta1.Seed) *SeedContext {
	s := &SeedContext{
		TestContext: *t,
		Seed:        seed,
	}
	s.Log = s.Log.WithValues("seed", client.ObjectKeyFromObject(seed))

	return s
}

// ManagedSeedContext is a test case-specific TestContext that carries test state and helpers through multiple steps of the
// same test case, i.e., within the same ordered container.
// Accordingly, ManagedSeedContext values must not be reused across multiple test cases (ordered containers). Make sure to
// declare ManagedSeedContext variables within the ordered container and initialize them during ginkgo tree construction,
// e.g., in a BeforeTestSetup node or when invoking a shared `test` func.
//
// A ManagedSeedContext can be initialized using TestContext.ForManagedSeed.
type ManagedSeedContext struct {
	TestContext

	// ManagedSeed object the test is working with
	ManagedSeed *seedmanagementv1alpha1.ManagedSeed

	// ShootContext object the managed seed is referencing
	ShootContext *ShootContext

	// Seed object the managed seed is referencing
	SeedContext *SeedContext
}

// ForManagedSeed copies the receiver ShootContext for deriving a ManagedSeedContext.
func (t *TestContext) ForManagedSeed(baseShoot *gardencorev1beta1.Shoot, managedSeed *seedmanagementv1alpha1.ManagedSeed) *ManagedSeedContext {
	seed := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name: managedSeed.Name,
		},
	}

	ms := &ManagedSeedContext{
		TestContext:  *t,
		ManagedSeed:  managedSeed,
		SeedContext:  t.ForSeed(seed),
		ShootContext: t.ForShoot(baseShoot),
	}

	t.Log = t.Log.WithValues("managedSeed", client.ObjectKeyFromObject(managedSeed))

	return ms
}
