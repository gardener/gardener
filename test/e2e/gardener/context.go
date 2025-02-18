// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
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

	gardenClientSet, err := kubernetes.NewClientFromFile("", os.Getenv("KUBECONFIG"),
		kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.GardenScheme}),
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
// same test case, i.e., within the same ordered container. Accordingly, ShootContext values must not be reused across
// multiple test cases (ordered containers).
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
