// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	resourcesv1alpha1helper "github.com/gardener/gardener/pkg/api/resources/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/logger"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	utilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
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

			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(policy), &admissionregistrationv1.ValidatingAdmissionPolicy{})).To(Succeed())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(policyBinding), &admissionregistrationv1.ValidatingAdmissionPolicyBinding{})).To(BeNotFoundError())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(systemBinding), &admissionregistrationv1.ValidatingAdmissionPolicyBinding{})).To(Succeed())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(noCleanupBinding), &admissionregistrationv1.ValidatingAdmissionPolicyBinding{})).To(Succeed())
		})

		It("should delete webhook configurations but keep system-component and no-cleanup ones", func() {
			Expect(botanist.CleanWebhooks(ctx)).To(Succeed())

			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(mutatingWebhook), &admissionregistrationv1.MutatingWebhookConfiguration{})).To(BeNotFoundError())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(systemMutatingWebhook), &admissionregistrationv1.MutatingWebhookConfiguration{})).To(Succeed())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(noCleanupMutatingWebhook), &admissionregistrationv1.MutatingWebhookConfiguration{})).To(Succeed())

			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(validatingWebhook), &admissionregistrationv1.ValidatingWebhookConfiguration{})).To(BeNotFoundError())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(systemValidatingWebhook), &admissionregistrationv1.ValidatingWebhookConfiguration{})).To(Succeed())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(noCleanupValidatingWebhook), &admissionregistrationv1.ValidatingWebhookConfiguration{})).To(Succeed())
		})
	})

	Describe("#CleanExtendedAPIs", func() {
		var (
			ctx         = context.Background()
			shootClient client.Client
			botanist    *Botanist

			apiService, systemAPIService, autoManagedAPIService, crd, noCleanupCRD client.Object
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

			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(apiService), &apiregistrationv1.APIService{})).To(BeNotFoundError())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(systemAPIService), &apiregistrationv1.APIService{})).To(Succeed())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(autoManagedAPIService), &apiregistrationv1.APIService{})).To(Succeed())

			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(crd), &apiextensionsv1.CustomResourceDefinition{})).To(BeNotFoundError())
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

			controlPlaneNamespace = gardenerutils.ComputeTechnicalID("foo", &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: "bar"}})

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

			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{})).To(BeNotFoundError())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(systemDeployment), &appsv1.Deployment{})).To(Succeed())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(noCleanupDeployment), &appsv1.Deployment{})).To(Succeed())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(service), &corev1.Service{})).To(BeNotFoundError())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(pvc), &corev1.PersistentVolumeClaim{})).To(BeNotFoundError())
		})

		It("should only consider Services and PersistentVolumeClaims due to skip-cleanup is set", func() {
			shoot := botanist.Shoot.GetInfo().DeepCopy()
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationShootSkipCleanup, "true")
			botanist.Shoot.SetInfo(shoot)

			Expect(botanist.CleanKubernetesResources(ctx)).To(Succeed())

			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(deployment), &appsv1.Deployment{})).To(Succeed())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(service), &corev1.Service{})).To(BeNotFoundError())
			Expect(shootClient.Get(ctx, client.ObjectKeyFromObject(pvc), &corev1.PersistentVolumeClaim{})).To(BeNotFoundError())
		})
	})

	Describe("#ignoreUnknownNamespaces", func() {
		It("should ignore objects in unknown namespaces", func() {
			ignore := IgnoreUnknownNamespaces([]string{"test1", "test2"})

			Expect(ignore(logr.Discard(), &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "test1"}})).To(BeFalse())
			Expect(ignore(logr.Discard(), &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "test2"}})).To(BeFalse())
			Expect(ignore(logr.Discard(), &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: ""}})).To(BeFalse())
			Expect(ignore(logr.Discard(), &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "test3"}})).To(BeTrue())
		})

		It("should not ignore cluster-scoped objects", func() {
			ignore := IgnoreUnknownNamespaces([]string{"test1", "test2"})

			Expect(ignore(logr.Discard(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test3"}})).To(BeFalse())
		})
	})

	Describe("#ignoreObjectsCreatedAfter", func() {
		It("should ignore objects created after the configured time", func() {
			now := testclock.NewFakeClock(time.Now()).Now()
			ignore := IgnoreObjectsCreatedAfter(now)

			Expect(ignore(logr.Discard(), &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.Time{Time: now.Add(-time.Second)}}})).To(BeFalse())
			Expect(ignore(logr.Discard(), &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.Time{Time: now}}})).To(BeFalse())
			Expect(ignore(logr.Discard(), &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.Time{Time: now.Add(time.Second)}}})).To(BeTrue())
		})
	})

	Describe("#ignoreObjectsManagedByManagedResourceInNamespace", func() {
		const (
			clusterID = "some-cluster-id"
			mrName    = "my-mr"
		)

		var (
			ctx             = context.Background()
			seedClient      client.Client
			managedResource *resourcesv1alpha1.ManagedResource
			ignore          utilclient.IgnoreLeftoverFunc
			controlPlaneNS  = gardenerutils.ComputeTechnicalID("foo", &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: "bar"}})
		)

		BeforeEach(func() {
			managedResource = &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: controlPlaneNS, Name: mrName}}
			seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithObjects(managedResource).Build()
			ignore = IgnoreObjectsManagedByManagedResourceInNamespace(ctx, seedClient, controlPlaneNS)
		})

		It("should not ignore objects without the origin annotation", func() {
			Expect(ignore(logr.Discard(), &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm", Namespace: "default"}})).To(BeFalse())
		})

		It("should no longer ignore objects after their ManagedResource is deleted", func() {
			obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{resourcesv1alpha1.OriginAnnotation: resourcesv1alpha1helper.OriginForManagedResource(clusterID, managedResource)},
			}}
			Expect(ignore(logr.Discard(), obj)).To(BeTrue())
			Expect(seedClient.Delete(ctx, managedResource)).To(Succeed())
			Expect(ignore(logr.Discard(), obj)).To(BeFalse())
		})

		It("should not ignore objects when checking their ManagedResource fails", func() {
			seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).WithInterceptorFuncs(interceptor.Funcs{
				Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
					return errors.New("cannot get ManagedResource")
				},
			}).Build()
			ignore = IgnoreObjectsManagedByManagedResourceInNamespace(ctx, seedClient, controlPlaneNS)
			obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{resourcesv1alpha1.OriginAnnotation: resourcesv1alpha1helper.OriginForManagedResource(clusterID, managedResource)},
			}}
			Expect(ignore(logr.Discard(), obj)).To(BeFalse())
		})

		It("should not ignore objects whose origin points to a ManagedResource in a different namespace", func() {
			obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name:      "cm",
				Namespace: "default",
				Annotations: map[string]string{
					resourcesv1alpha1.OriginAnnotation: resourcesv1alpha1helper.OriginForManagedResource(clusterID, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: "other-namespace", Name: mrName}}),
				},
			}}
			Expect(ignore(logr.Discard(), obj)).To(BeFalse())
		})

		It("should ignore objects whose origin points to a ManagedResource in the control-plane namespace", func() {
			obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name:      "cm",
				Namespace: "default",
				Annotations: map[string]string{
					resourcesv1alpha1.OriginAnnotation: resourcesv1alpha1helper.OriginForManagedResource("", &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: controlPlaneNS, Name: mrName}}),
				},
			}}
			Expect(ignore(logr.Discard(), obj)).To(BeTrue())
		})
	})
})

var _ = Describe("Cleanup of control plane namespaces", func() {
	var (
		fakeClient client.Client
		botanist   *Botanist
		shootInfo  *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		// The fake client only supports exact-match field selectors. Evaluate the field selectors of the cleanup like
		// the kube-apiserver does instead.
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				listOpts := (&client.ListOptions{}).ApplyOptions(opts)
				fieldSelector := listOpts.FieldSelector
				listOpts.FieldSelector = nil

				if err := c.List(ctx, list, listOpts); err != nil || fieldSelector == nil {
					return err
				}

				var items []runtime.Object
				if err := meta.EachListItem(list, func(o runtime.Object) error {
					obj := o.(client.Object)
					if fieldSelector.Matches(fields.Set{MetadataNameField: obj.GetName(), MetadataNamespaceField: obj.GetNamespace()}) {
						items = append(items, o)
					}
					return nil
				}); err != nil {
					return err
				}
				return meta.SetList(list, items)
			},
		}).Build()
		fakeClientSet := fake.NewClientSetBuilder().WithClient(fakeClient).Build()

		shootInfo = &gardencorev1beta1.Shoot{}

		botanist = &Botanist{Operation: &operation.Operation{
			Logger:              logr.Discard(),
			Clock:               testclock.NewFakeClock(metav1.Now().Time),
			SeedClientSet:       fakeClientSet,
			SeedNamespaceObject: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
			ShootClientSet:      fakeClientSet,
			Shoot:               &shootpkg.Shoot{ControlPlaneNamespace: "kube-system"},
		}}
		botanist.Shoot.SetInfo(shootInfo)
	})

	JustBeforeEach(func(ctx SpecContext) {
		for _, obj := range []client.Object{
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "garden"}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "extension-foo", Labels: map[string]string{"gardener.cloud/role": "extension"}}},
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "control-plane"}},
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "garden", Name: "gardener-resource-manager"}},
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "extension-foo", Name: "extension"}},
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "workload"}},
			&corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "control-plane"}},
			&corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "workload"}},
			&admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "control-plane"},
				Webhooks:   []admissionregistrationv1.MutatingWebhook{{ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: &admissionregistrationv1.ServiceReference{Namespace: "kube-system", Name: "foo"}}}},
			},
			&admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager"},
				Webhooks:   []admissionregistrationv1.MutatingWebhook{{ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: &admissionregistrationv1.ServiceReference{Namespace: "garden", Name: "foo"}}}},
			},
			&admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "workload"},
				Webhooks:   []admissionregistrationv1.MutatingWebhook{{ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: &admissionregistrationv1.ServiceReference{Namespace: "default", Name: "foo"}}}},
			},
			&admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "extension"},
				Webhooks:   []admissionregistrationv1.ValidatingWebhook{{ClientConfig: admissionregistrationv1.WebhookClientConfig{Service: &admissionregistrationv1.ServiceReference{Namespace: "extension-foo", Name: "foo"}}}},
			},
			&admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "workload"},
				Webhooks:   []admissionregistrationv1.ValidatingWebhook{{ClientConfig: admissionregistrationv1.WebhookClientConfig{URL: new("https://foo")}}},
			},
			&apiregistrationv1.APIService{
				ObjectMeta: metav1.ObjectMeta{Name: "v1.control-plane"},
				Spec:       apiregistrationv1.APIServiceSpec{Service: &apiregistrationv1.ServiceReference{Namespace: "kube-system", Name: "foo"}},
			},
			&apiregistrationv1.APIService{
				ObjectMeta: metav1.ObjectMeta{Name: "v1.workload"},
				Spec:       apiregistrationv1.APIServiceSpec{Service: &apiregistrationv1.ServiceReference{Namespace: "default", Name: "foo"}},
			},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "control-plane", Labels: map[string]string{"gardener.cloud/deletion-protected": "true"}}},
			&apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "workload"}},
		} {
			Expect(fakeClient.Create(ctx, obj)).To(Succeed())
		}
	})

	remainingNames := func(ctx context.Context, list client.ObjectList) []string {
		ExpectWithOffset(1, fakeClient.List(ctx, list)).To(Succeed())

		var names []string
		ExpectWithOffset(1, meta.EachListItem(list, func(o runtime.Object) error {
			obj := o.(client.Object)
			names = append(names, obj.GetNamespace()+"/"+obj.GetName())
			return nil
		})).To(Succeed())
		return names
	}

	When("the shoot is hosted", func() {
		It("should clean up all resources", func(ctx SpecContext) {
			Expect(botanist.CleanWebhooks(ctx)).To(Succeed())
			Expect(botanist.CleanExtendedAPIs(ctx)).To(Succeed())
			Expect(botanist.CleanKubernetesResources(ctx)).To(Succeed())

			Expect(remainingNames(ctx, &corev1.PodList{})).To(BeEmpty())
			Expect(remainingNames(ctx, &corev1.ServiceList{})).To(BeEmpty())
			Expect(remainingNames(ctx, &admissionregistrationv1.MutatingWebhookConfigurationList{})).To(BeEmpty())
			Expect(remainingNames(ctx, &admissionregistrationv1.ValidatingWebhookConfigurationList{})).To(BeEmpty())
			Expect(remainingNames(ctx, &apiregistrationv1.APIServiceList{})).To(BeEmpty())
			Expect(remainingNames(ctx, &apiextensionsv1.CustomResourceDefinitionList{})).To(BeEmpty())
		})
	})

	When("the shoot is self-hosted", func() {
		BeforeEach(func() {
			shootInfo.Spec.Provider.Workers = []gardencorev1beta1.Worker{{ControlPlane: &gardencorev1beta1.WorkerControlPlane{}}}
		})

		It("should skip the resources of the control plane", func(ctx SpecContext) {
			Expect(botanist.CleanWebhooks(ctx)).To(Succeed())
			Expect(botanist.CleanExtendedAPIs(ctx)).To(Succeed())
			Expect(botanist.CleanKubernetesResources(ctx)).To(Succeed())

			Expect(remainingNames(ctx, &corev1.PodList{})).To(ConsistOf("kube-system/control-plane", "garden/gardener-resource-manager", "extension-foo/extension"))
			Expect(remainingNames(ctx, &corev1.ServiceList{})).To(ConsistOf("kube-system/control-plane"))
			Expect(remainingNames(ctx, &admissionregistrationv1.MutatingWebhookConfigurationList{})).To(ConsistOf("/control-plane", "/gardener-resource-manager"))
			Expect(remainingNames(ctx, &admissionregistrationv1.ValidatingWebhookConfigurationList{})).To(ConsistOf("/extension"))
			Expect(remainingNames(ctx, &apiregistrationv1.APIServiceList{})).To(ConsistOf("/v1.control-plane"))
			Expect(remainingNames(ctx, &apiextensionsv1.CustomResourceDefinitionList{})).To(ConsistOf("/control-plane"))
		})
	})
})
