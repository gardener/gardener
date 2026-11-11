// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/logger"
)

var _ = Describe("Cleanup", func() {
	Describe("#CleanWebhooks", func() {
		var (
			ctx         = context.Background()
			shootClient client.Client
			botanist    *Botanist

			policy, policyBinding, systemBinding, noCleanupBinding                 client.Object
			mutatingWebhook, systemMutatingWebhook, noCleanupMutatingWebhook       client.Object
			validatingWebhook, systemValidatingWebhook, noCleanupValidatingWebhook client.Object
		)

		BeforeEach(func() {
			shootClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()

			s := &shootpkg.Shoot{}
			s.SetInfo(&gardencorev1beta1.Shoot{})

			botanist = &Botanist{Operation: &operation.Operation{
				Logger:         logger.MustNewZapLogger(logger.InfoLevel, logger.FormatJSON),
				ShootClientSet: fake.NewClientSetBuilder().WithClient(shootClient).Build(),
				Shoot:          s,
			}}

			policy = &admissionregistrationv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: "policy"}}

			policyBinding = &admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: "binding"}}
			systemBinding = &admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{
				Name:   "system-binding",
				Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSystemComponent},
			}}
			noCleanupBinding = &admissionregistrationv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{
				Name:   "no-cleanup-binding",
				Labels: map[string]string{v1beta1constants.ShootNoCleanup: "true"},
			}}

			mutatingWebhook = &admissionregistrationv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "mutating"}}
			systemMutatingWebhook = &admissionregistrationv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{
				Name:   "system-mutating",
				Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSystemComponent},
			}}
			noCleanupMutatingWebhook = &admissionregistrationv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{
				Name:   "no-cleanup-mutating",
				Labels: map[string]string{v1beta1constants.ShootNoCleanup: "true"},
			}}

			validatingWebhook = &admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "validating"}}
			systemValidatingWebhook = &admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{
				Name:   "system-validating",
				Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSystemComponent},
			}}
			noCleanupValidatingWebhook = &admissionregistrationv1.ValidatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{
				Name:   "no-cleanup-validating",
				Labels: map[string]string{v1beta1constants.ShootNoCleanup: "true"},
			}}

			for _, obj := range []client.Object{
				policy, policyBinding, systemBinding, noCleanupBinding,
				mutatingWebhook, systemMutatingWebhook, noCleanupMutatingWebhook,
				validatingWebhook, systemValidatingWebhook, noCleanupValidatingWebhook,
			} {
				Expect(shootClient.Create(ctx, obj)).To(Succeed())
			}
		})

		It("should delete ValidatingAdmissionPolicyBindings but keep protected ones and the policies", func() {
			Expect(botanist.CleanWebhooks(ctx)).To(Succeed())

			Expect(apierrors.IsNotFound(shootClient.Get(ctx, client.ObjectKeyFromObject(policyBinding), &admissionregistrationv1.ValidatingAdmissionPolicyBinding{}))).To(BeTrue())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(systemBinding), &admissionregistrationv1.ValidatingAdmissionPolicyBinding{})).To(Succeed())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(noCleanupBinding), &admissionregistrationv1.ValidatingAdmissionPolicyBinding{})).To(Succeed())
		})

		It("should delete webhook configurations but keep system-component and no-cleanup ones", func() {
			Expect(botanist.CleanWebhooks(ctx)).To(Succeed())

			Expect(apierrors.IsNotFound(shootClient.Get(ctx, client.ObjectKeyFromObject(mutatingWebhook), &admissionregistrationv1.MutatingWebhookConfiguration{}))).To(BeTrue())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(systemMutatingWebhook), &admissionregistrationv1.MutatingWebhookConfiguration{})).To(Succeed())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(noCleanupMutatingWebhook), &admissionregistrationv1.MutatingWebhookConfiguration{})).To(Succeed())

			Expect(apierrors.IsNotFound(shootClient.Get(ctx, client.ObjectKeyFromObject(validatingWebhook), &admissionregistrationv1.ValidatingWebhookConfiguration{}))).To(BeTrue())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(systemValidatingWebhook), &admissionregistrationv1.ValidatingWebhookConfiguration{})).To(Succeed())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(noCleanupValidatingWebhook), &admissionregistrationv1.ValidatingWebhookConfiguration{})).To(Succeed())
		})
	})

	Describe("#CleanExtendedAPIs", func() {
		var (
			ctx         = context.Background()
			shootClient client.Client
			botanist    *Botanist

			apiService, noCleanupAPIService, systemAPIService, autoManagedAPIService, crd, noCleanupCRD client.Object
		)

		BeforeEach(func() {
			shootClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()

			s := &shootpkg.Shoot{}
			s.SetInfo(&gardencorev1beta1.Shoot{})

			botanist = &Botanist{Operation: &operation.Operation{
				Logger:         logger.MustNewZapLogger(logger.InfoLevel, logger.FormatJSON),
				ShootClientSet: fake.NewClientSetBuilder().WithClient(shootClient).Build(),
				Shoot:          s,
			}}

			apiService = &apiregistrationv1.APIService{ObjectMeta: metav1.ObjectMeta{Name: "v1.example.com"}}
			systemAPIService = &apiregistrationv1.APIService{ObjectMeta: metav1.ObjectMeta{
				Name:   "v1.system.example.com",
				Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSystemComponent},
			}}
			noCleanupAPIService = &apiregistrationv1.APIService{ObjectMeta: metav1.ObjectMeta{
				Name:   "v1.system.example.com",
				Labels: map[string]string{v1beta1constants.ShootNoCleanup: "true"},
			}}
			autoManagedAPIService = &apiregistrationv1.APIService{ObjectMeta: metav1.ObjectMeta{
				Name: "v1.automanaged.example.com",
				// The cleanup selector protects APIServices carrying the kube-aggregator automanaged label regardless
				// of its value, hence any non-empty value works here.
				Labels: map[string]string{KubeAggregatorAutoManaged: "onstart"},
			}}

			crd = &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "foos.example.com"}}
			noCleanupCRD = &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{
				Name:   "bars.example.com",
				Labels: map[string]string{v1beta1constants.ShootNoCleanup: "true"},
			}}

			for _, obj := range []client.Object{apiService, systemAPIService, autoManagedAPIService, crd, noCleanupCRD} {
				Expect(shootClient.Create(ctx, obj)).To(Succeed())
			}
		})

		It("should delete APIServices and CRDs but keep protected ones", func() {
			Expect(botanist.CleanExtendedAPIs(ctx)).To(Succeed())

			Expect(apierrors.IsNotFound(shootClient.Get(ctx, client.ObjectKeyFromObject(apiService), &apiregistrationv1.APIService{}))).To(BeTrue())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(noCleanupAPIService), &apiregistrationv1.APIService{})).To(Succeed())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(systemAPIService), &apiregistrationv1.APIService{})).To(Succeed())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(autoManagedAPIService), &apiregistrationv1.APIService{})).To(Succeed())

			Expect(apierrors.IsNotFound(shootClient.Get(ctx, client.ObjectKeyFromObject(crd), &apiextensionsv1.CustomResourceDefinition{}))).To(BeTrue())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(noCleanupCRD), &apiextensionsv1.CustomResourceDefinition{})).To(Succeed())
		})
	})

	Describe("#CleanKubernetesResources", func() {
		var (
			ctx         = context.Background()
			shootClient client.Client
			seedClient  client.Client
			botanist    *Botanist
			fakeClock   *testclock.FakeClock

			controlPlaneNamespace = "shoot--foo--bar"

			deployment, systemDeployment, noCleanupDeployment, service, pvc client.Object
		)

		BeforeEach(func() {
			shootClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
			seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			fakeClock = testclock.NewFakeClock(time.Now())

			seedNamespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: controlPlaneNamespace}}
			Expect(seedClient.Create(ctx, seedNamespace)).To(Succeed())

			s := &shootpkg.Shoot{ControlPlaneNamespace: controlPlaneNamespace}
			s.SetInfo(&gardencorev1beta1.Shoot{})

			botanist = &Botanist{Operation: &operation.Operation{
				Logger:              logger.MustNewZapLogger(logger.InfoLevel, logger.FormatJSON),
				ShootClientSet:      fake.NewClientSetBuilder().WithClient(shootClient).Build(),
				SeedClientSet:       fake.NewClientSetBuilder().WithClient(seedClient).Build(),
				SeedNamespaceObject: seedNamespace,
				Shoot:               s,
				Clock:               fakeClock,
			}}

			deployment = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "deploy", Namespace: "default"}}
			systemDeployment = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      "system-deploy",
				Namespace: "default",
				Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSystemComponent},
			}}
			noCleanupDeployment = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      "no-cleanup-deploy",
				Namespace: "default",
				Labels:    map[string]string{v1beta1constants.ShootNoCleanup: "true"},
			}}

			service = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "svc", Namespace: "default"}}

			pvc = &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "pvc", Namespace: "default"}}

			namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}

			Expect(shootClient.Create(ctx, namespace)).To(Succeed())

			for _, obj := range []client.Object{deployment, systemDeployment, noCleanupDeployment, service, pvc} {
				Expect(shootClient.Create(ctx, obj)).To(Succeed())
			}

			// Advance the clock so that the recorded cleanup start time plus the finalize grace period lies in the
			// future relative to the objects' creation timestamps, otherwise they would be ignored as leftovers.
			fakeClock.Step(2 * time.Hour)
		})

		It("should delete the Kubernetes resources but keep protected ones", func() {
			Expect(botanist.CleanKubernetesResources(ctx)).To(Succeed())

			Expect(apierrors.IsNotFound(shootClient.Get(ctx, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{}))).To(BeTrue())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(systemDeployment), &appsv1.Deployment{})).To(Succeed())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(noCleanupDeployment), &appsv1.Deployment{})).To(Succeed())
			Expect(apierrors.IsNotFound(shootClient.Get(ctx, client.ObjectKeyFromObject(service), &corev1.Service{}))).To(BeTrue())
			Expect(apierrors.IsNotFound(shootClient.Get(ctx, client.ObjectKeyFromObject(pvc), &corev1.PersistentVolumeClaim{}))).To(BeTrue())
		})

		It("should only consider Services and PersistentVolumeClaims due to skip-cleanup is set", func() {
			shoot := botanist.Shoot.GetInfo().DeepCopy()
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationShootSkipCleanup, "true")
			botanist.Shoot.SetInfo(shoot)

			Expect(botanist.CleanKubernetesResources(ctx)).To(Succeed())

			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{})).To(Succeed())
			Expect(apierrors.IsNotFound(shootClient.Get(ctx, client.ObjectKeyFromObject(service), &corev1.Service{}))).To(BeTrue())
			Expect(apierrors.IsNotFound(shootClient.Get(ctx, client.ObjectKeyFromObject(pvc), &corev1.PersistentVolumeClaim{}))).To(BeTrue())
		})
	})
})
