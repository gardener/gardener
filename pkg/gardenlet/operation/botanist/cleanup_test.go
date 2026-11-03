// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("Cleanup", func() {
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
		fakeClientSet := fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build()

		shootInfo = &gardencorev1beta1.Shoot{}

		botanist = &Botanist{Operation: &operation.Operation{
			Logger:              logr.Discard(),
			Clock:               testclock.NewFakeClock(metav1.Now().Time),
			SeedClientSet:       fakeClientSet,
			SeedNamespaceObject: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
			ShootClientSet:      fakeClientSet,
			Shoot:               &shoot.Shoot{ControlPlaneNamespace: "kube-system"},
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
