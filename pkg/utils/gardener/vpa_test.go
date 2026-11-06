// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("VPA", func() {
	var (
		ctx        context.Context
		fakeClient client.Client

		name      = "foo"
		namespace = "bar"

		vpa *vpaautoscalingv1.VerticalPodAutoscaler
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		ctx = context.Background()

		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: name + "-vpa", Namespace: namespace}}
		Expect(fakeClient.Create(ctx, vpa)).To(Succeed())
	})

	Describe("#ReconcileVPAForGardenerComponent", func() {
		It("should reconcile the VPA successfully", func() {
			Expect(ReconcileVPAForGardenerComponent(ctx, fakeClient, name, namespace)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(Succeed())
			Expect(vpa).To(Equal(&vpaautoscalingv1.VerticalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:            name + "-vpa",
					Namespace:       namespace,
					ResourceVersion: "2",
				},
				Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
					TargetRef: &autoscalingv1.CrossVersionObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       name,
					},
					UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
						UpdateMode: new(vpaautoscalingv1.UpdateModeInPlaceOrRecreate),
					},
					ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
						ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
							{
								ContainerName: name,
								MinAllowed: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("200Mi"),
								},
							},
							{
								ContainerName: "*",
								Mode:          new(vpaautoscalingv1.ContainerScalingModeOff),
							},
						},
					},
				},
			}))
		})
	})

	Describe("#DeleteVPAForGardenerComponent", func() {
		It("should delete the VPA successfully", func() {
			Expect(DeleteVPAForGardenerComponent(ctx, fakeClient, name, namespace)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(vpa), vpa)).To(BeNotFoundError())
		})

		It("should succeed when CRD does not exist (NoMatchError)", func() {
			noMatchErr := &meta.NoKindMatchError{
				GroupKind:        schema.GroupKind{Group: "test.gardener.cloud", Kind: "TestResource"},
				SearchedVersions: []string{"v1alpha1"},
			}

			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetesscheme.Scheme).
				WithInterceptorFuncs(interceptor.Funcs{
					Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
						return noMatchErr
					},
				}).
				Build()

			Expect(DeleteVPAForGardenerComponent(ctx, fakeClient, name, namespace)).To(Succeed())
		})
	})

	Describe("#RemoveVPAInPlaceUpdatesMutatedLabel", func() {
		const mutatedLabel = "vpa-in-place-updates.resources.gardener.cloud/mutated"

		newVPA := func(name, namespace string, labels map[string]string) *vpaautoscalingv1.VerticalPodAutoscaler {
			return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels}}
		}

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		})

		It("should succeed when there are no VPAs", func() {
			Expect(RemoveVPAInPlaceUpdatesMutatedLabel(ctx, fakeClient, fakeClient)).To(Succeed())
		})

		It("should only remove the mutated label from VPAs across namespaces", func() {
			Expect(fakeClient.Create(ctx, newVPA("a", "ns1", map[string]string{mutatedLabel: "true", "foo": "bar"}))).To(Succeed())
			Expect(fakeClient.Create(ctx, newVPA("b", "ns2", map[string]string{mutatedLabel: "true"}))).To(Succeed())
			Expect(fakeClient.Create(ctx, newVPA("c", "ns2", map[string]string{"foo": "bar"}))).To(Succeed())

			Expect(RemoveVPAInPlaceUpdatesMutatedLabel(ctx, fakeClient, fakeClient)).To(Succeed())

			get := func(name, namespace string) map[string]string {
				v := &vpaautoscalingv1.VerticalPodAutoscaler{}
				ExpectWithOffset(1, fakeClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, v)).To(Succeed())
				return v.Labels
			}
			Expect(get("a", "ns1")).To(Equal(map[string]string{"foo": "bar"}))
			Expect(get("b", "ns2")).To(BeEmpty())
			Expect(get("c", "ns2")).To(Equal(map[string]string{"foo": "bar"}))
		})

		It("should succeed when the VPA CRD does not exist (NoMatchError)", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.SeedScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
						return &meta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"}}
					},
				}).
				Build()

			Expect(RemoveVPAInPlaceUpdatesMutatedLabel(ctx, fakeClient, fakeClient)).To(Succeed())
		})

		It("should return an error when listing fails", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.SeedScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
						return errors.New("fake list error")
					},
				}).
				Build()

			Expect(RemoveVPAInPlaceUpdatesMutatedLabel(ctx, fakeClient, fakeClient)).To(MatchError(ContainSubstring("fake list error")))
		})

		It("should return an error when patching fails", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.SeedScheme).
				WithObjects(newVPA("a", "ns1", map[string]string{mutatedLabel: "true"})).
				WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return errors.New("fake patch error")
					},
				}).
				Build()

			Expect(RemoveVPAInPlaceUpdatesMutatedLabel(ctx, fakeClient, fakeClient)).To(MatchError(And(ContainSubstring("ns1/a"), ContainSubstring("fake patch error"))))
		})

		It("should ignore VPAs that were deleted in the meantime", func() {
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.SeedScheme).
				WithObjects(newVPA("a", "ns1", map[string]string{mutatedLabel: "true"})).
				WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(_ context.Context, _ client.WithWatch, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return apierrors.NewNotFound(schema.GroupResource{Group: "autoscaling.k8s.io", Resource: "verticalpodautoscalers"}, obj.GetName())
					},
				}).
				Build()

			Expect(RemoveVPAInPlaceUpdatesMutatedLabel(ctx, fakeClient, fakeClient)).To(Succeed())
		})
	})
})
