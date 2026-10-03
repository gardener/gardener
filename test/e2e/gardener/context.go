// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
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

// NewTestContext returns a new TestContext for working with the garden cluster pointed to by the KUBECONFIG env var.
// The clients are not initialized yet, this must be done by calling Init. NewTestContext does not fail, so it is safe to
// call it during tree construction. Init on the other hand must be called in a setup node like BeforeAll.
func NewTestContext() *TestContext {
	return &TestContext{
		Log: logger.MustNewZapLogger(logger.DebugLevel, logger.FormatText, logzap.WriteTo(GinkgoWriter)),
	}
}

// Init initializes the garden clients of the TestContext. It panics if the initialization fails and should be called in
// a setup node like BeforeAll, not during tree construction.
func (t *TestContext) Init() *TestContext {
	gardenScheme := kubernetes.GardenScheme
	utilruntime.Must(operatorv1alpha1.AddToScheme(gardenScheme))
	utilruntime.Must(resourcesv1alpha1.AddToScheme(gardenScheme))
	utilruntime.Must(apiextensionsscheme.AddToScheme(gardenScheme))
	utilruntime.Must(monitoringv1.AddToScheme(gardenScheme))

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

// ShootContext is a test case-specific TestContext that carries test state and helpers through multiple steps of the
// same test case, i.e., within the same ordered container.
// Accordingly, ShootContext values must not be reused across multiple test cases (ordered containers). Make sure to
// declare ShootContext variables within the ordered container and initialize them in a BeforeAll node.
//
// A ShootContext is created using NewShootContext and initialized by calling Init in a BeforeAll node. In contrast to
// the other context types, the Shoot is set during tree construction because many shared helper functions decide which
// specs to register depending on the shoot's spec (e.g., whether it is workerless). Building the Shoot object cannot
// fail, so this is safe. Only the clients are initialized in the BeforeAll node.
type ShootContext struct {
	TestContext

	// SeedContext of the seed the shoot is scheduled to.
	*SeedContext

	// Shoot object that the test case is working with.
	Shoot *gardencorev1beta1.Shoot

	// ShootClientSet is a client for the shoot cluster. It must be initialized via SetShootClientSet.
	ShootClientSet kubernetes.Interface
	// ShootClient is the controller-runtime client of the ShootClientSet. This is a more convenient equivalent of
	// ShootClientSet.Client().
	ShootClient client.Client
	// ShootKomega is a Komega instance for writing assertions on objects in the shoot cluster. E.g.,
	//  Eventually(ctx, s.ShootKomega.ObjectList(&corev1.NodeList{})).Should(
	//    HaveField("Items", HaveLen(1)),
	//  )
	ShootKomega komega.Komega

	// ControlPlaneNamespace contains the namespace for the Shoot Control Plane in the Seed.
	// It must be initialized via SetControlPlaneNamespace.
	ControlPlaneNamespace string
}

// ForShoot copies the receiver TestContext for deriving a ShootContext.
func (t *TestContext) ForShoot(shoot *gardencorev1beta1.Shoot) *ShootContext {
	return (&ShootContext{TestContext: *t, SeedContext: &SeedContext{}}).SetShoot(shoot)
}

// NewShootContext returns a ShootContext for the given shoot. The clients are not initialized yet, this must be done in
// a BeforeAll node by calling Init. NewShootContext does not fail, so it is safe to call it during tree construction.
// This is needed because many shared helper functions decide which specs to register depending on the shoot's spec
// (e.g., whether it is workerless).
func NewShootContext(shoot *gardencorev1beta1.Shoot) *ShootContext {
	return (&ShootContext{TestContext: *NewTestContext(), SeedContext: &SeedContext{}}).SetShoot(shoot)
}

// Init initializes the garden clients of the ShootContext, see TestContext.Init.
func (t *ShootContext) Init() *ShootContext {
	t.TestContext.Init()
	return t
}

// SetShoot sets the Shoot of the ShootContext and adds it to the logger.
func (t *ShootContext) SetShoot(shoot *gardencorev1beta1.Shoot) *ShootContext {
	t.Shoot = shoot
	t.Log = t.Log.WithValues("shoot", client.ObjectKeyFromObject(shoot))
	return t
}

// SetShootClientSet initializes the shoot clients of this ShootContext from the given client set.
func (t *ShootContext) SetShootClientSet(clientSet kubernetes.Interface) *ShootContext {
	t.ShootClientSet = clientSet
	t.ShootClient = clientSet.Client()
	t.ShootKomega = komega.New(t.ShootClient)
	return t
}

// SetControlPlaneNamespace sets the namespace for the Shoot Control Plane in the Seed.
func (t *ShootContext) SetControlPlaneNamespace(namespace string) *ShootContext {
	t.ControlPlaneNamespace = namespace
	return t
}

// ProjectContext is a test case-specific TestContext that carries test state and helpers through multiple steps of the
// same test case, i.e., within the same ordered container.
// Accordingly, ProjectContext values must not be reused across multiple test cases (ordered containers). Make sure to
// declare ProjectContext variables within the ordered container and initialize them in a BeforeAll node.
//
// A ProjectContext is created using NewProjectContext and initialized by calling Init followed by SetProject in a
// BeforeAll node.
type ProjectContext struct {
	TestContext

	Project *gardencorev1beta1.Project
}

// NewProjectContext returns an empty ProjectContext. The clients are not initialized yet, this must be done in a
// BeforeAll node by calling Init, followed by SetProject.
func NewProjectContext() *ProjectContext {
	return &ProjectContext{TestContext: *NewTestContext()}
}

// Init initializes the garden clients of the ProjectContext, see TestContext.Init.
func (t *ProjectContext) Init() *ProjectContext {
	t.TestContext.Init()
	return t
}

// SetProject sets the Project of the ProjectContext and adds it to the logger.
func (t *ProjectContext) SetProject(project *gardencorev1beta1.Project) *ProjectContext {
	t.Project = project
	t.Log = t.Log.WithValues("project", client.ObjectKeyFromObject(project))
	return t
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
// declare SeedContext variables within the ordered container and initialize them in a BeforeAll node.
//
// A SeedContext is created using NewSeedContext and initialized by calling Init followed by SetSeed in a BeforeAll node.
// Alternatively, it can be derived from an initialized TestContext using TestContext.ForSeed.
type SeedContext struct {
	TestContext

	// Seed object the test is working with
	Seed *gardencorev1beta1.Seed

	// SeedClientSet is a client for the seed cluster. It must be initialized via SetSeedClientSet.
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

// ForSeed copies the receiver TestContext for deriving a SeedContext.
func (t *TestContext) ForSeed(seed *gardencorev1beta1.Seed) *SeedContext {
	return (&SeedContext{TestContext: *t}).SetSeed(seed)
}

// NewSeedContext returns an empty SeedContext. The clients are not initialized yet, this must be done in a BeforeAll
// node by calling Init, followed by SetSeed.
func NewSeedContext() *SeedContext {
	return &SeedContext{TestContext: *NewTestContext()}
}

// Init initializes the garden clients of the SeedContext, see TestContext.Init.
func (t *SeedContext) Init() *SeedContext {
	t.TestContext.Init()
	return t
}

// SetSeed sets the Seed of the SeedContext and adds it to the logger.
func (t *SeedContext) SetSeed(seed *gardencorev1beta1.Seed) *SeedContext {
	t.Seed = seed
	t.Log = t.Log.WithValues("seed", client.ObjectKeyFromObject(seed))

	return t
}

// SetSeedClientSet initializes the seed clients of this SeedContext from the given client set.
func (t *SeedContext) SetSeedClientSet(clientSet kubernetes.Interface) *SeedContext {
	t.SeedClientSet = clientSet
	t.SeedClient = clientSet.Client()
	t.SeedKomega = komega.New(t.SeedClient)
	return t
}

// ManagedSeedContext is a test case-specific TestContext that carries test state and helpers through multiple steps of the
// same test case, i.e., within the same ordered container.
// Accordingly, ManagedSeedContext values must not be reused across multiple test cases (ordered containers). Make sure to
// declare ManagedSeedContext variables within the ordered container and initialize them in a BeforeAll node.
//
// A ManagedSeedContext is created using NewManagedSeedContext and initialized by calling Init followed by
// SetManagedSeed in a BeforeAll node.
type ManagedSeedContext struct {
	TestContext

	// ManagedSeed object the test is working with
	ManagedSeed *seedmanagementv1alpha1.ManagedSeed

	// ShootContext of the shoot the managed seed is referencing
	ShootContext *ShootContext

	// SeedContext of the seed the managed seed is referencing
	SeedContext *SeedContext
}

// NewManagedSeedContext returns an empty ManagedSeedContext. It is safe to call during tree construction, i.e., the
// ShootContext and SeedContext can already be passed to shared helper functions. The context must be initialized in a
// BeforeAll node by calling Init, followed by SetManagedSeed.
func NewManagedSeedContext() *ManagedSeedContext {
	return &ManagedSeedContext{
		TestContext:  *NewTestContext(),
		ShootContext: &ShootContext{},
		SeedContext:  &SeedContext{},
	}
}

// Init initializes the garden clients of the ManagedSeedContext, see TestContext.Init.
func (t *ManagedSeedContext) Init() *ManagedSeedContext {
	t.TestContext.Init()
	return t
}

// SetManagedSeed sets the ManagedSeed of the ManagedSeedContext and derives the ShootContext and SeedContext from it.
// It must be called after Init.
func (t *ManagedSeedContext) SetManagedSeed(baseShoot *gardencorev1beta1.Shoot, managedSeed *seedmanagementv1alpha1.ManagedSeed) *ManagedSeedContext {
	seed := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name: managedSeed.Name,
		},
	}

	t.ManagedSeed = managedSeed
	*t.SeedContext = *t.ForSeed(seed)
	*t.ShootContext = *t.ForShoot(baseShoot)
	t.Log = t.Log.WithValues("managedSeed", client.ObjectKeyFromObject(managedSeed))

	return t
}
