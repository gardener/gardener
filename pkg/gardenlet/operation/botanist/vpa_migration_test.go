// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"errors"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
)

var _ = Describe("VPAMigration", func() {
	const mutatedLabel = "vpa-in-place-updates.resources.gardener.cloud/mutated"

	var (
		ctx      = context.Background()
		botanist *Botanist
	)

	newVPA := func(name, namespace string, labels map[string]string) *vpaautoscalingv1.VerticalPodAutoscaler {
		return &vpaautoscalingv1.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels}}
	}

	withClient := func(c client.Client) {
		botanist = &Botanist{Operation: &operation.Operation{
			ShootClientSet: fakekubernetes.NewClientSetBuilder().WithClient(c).Build(),
			Logger:         logr.Discard(),
		}}
	}

	Describe("#RemoveVPAInPlaceUpdatesMutatedLabel", func() {
		It("should succeed when there are no VPAs", func() {
			withClient(fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build())

			Expect(botanist.RemoveVPAInPlaceUpdatesMutatedLabel(ctx)).To(Succeed())
		})

		It("should only remove the mutated label from VPAs in kube-system and not touch other namespaces", func() {
			shootClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()
			withClient(shootClient)

			Expect(shootClient.Create(ctx, newVPA("a", metav1.NamespaceSystem, map[string]string{mutatedLabel: "true", "foo": "bar"}))).To(Succeed())
			Expect(shootClient.Create(ctx, newVPA("b", metav1.NamespaceSystem, map[string]string{mutatedLabel: "true"}))).To(Succeed())
			Expect(shootClient.Create(ctx, newVPA("c", metav1.NamespaceSystem, map[string]string{"foo": "bar"}))).To(Succeed())
			Expect(shootClient.Create(ctx, newVPA("d", "other-ns", map[string]string{mutatedLabel: "true"}))).To(Succeed())

			Expect(botanist.RemoveVPAInPlaceUpdatesMutatedLabel(ctx)).To(Succeed())

			get := func(name, namespace string) map[string]string {
				v := &vpaautoscalingv1.VerticalPodAutoscaler{}
				ExpectWithOffset(1, shootClient.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, v)).To(Succeed())
				return v.Labels
			}
			Expect(get("a", metav1.NamespaceSystem)).To(Equal(map[string]string{"foo": "bar"}))
			Expect(get("b", metav1.NamespaceSystem)).To(BeEmpty())
			Expect(get("c", metav1.NamespaceSystem)).To(Equal(map[string]string{"foo": "bar"}))
			// VPA in other namespace must not be touched
			Expect(get("d", "other-ns")).To(Equal(map[string]string{mutatedLabel: "true"}))
		})

		It("should succeed when the VPA CRD does not exist (NoMatchError)", func() {
			withClient(fakeclient.NewClientBuilder().
				WithScheme(kubernetes.ShootScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
						return &meta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"}}
					},
				}).
				Build())

			Expect(botanist.RemoveVPAInPlaceUpdatesMutatedLabel(ctx)).To(Succeed())
		})

		It("should return an error when listing fails", func() {
			withClient(fakeclient.NewClientBuilder().
				WithScheme(kubernetes.ShootScheme).
				WithInterceptorFuncs(interceptor.Funcs{
					List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
						return errors.New("fake list error")
					},
				}).
				Build())

			Expect(botanist.RemoveVPAInPlaceUpdatesMutatedLabel(ctx)).To(MatchError(ContainSubstring("fake list error")))
		})

		It("should return an error when patching fails", func() {
			withClient(fakeclient.NewClientBuilder().
				WithScheme(kubernetes.ShootScheme).
				WithObjects(newVPA("a", metav1.NamespaceSystem, map[string]string{mutatedLabel: "true"})).
				WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return errors.New("fake patch error")
					},
				}).
				Build())

			Expect(botanist.RemoveVPAInPlaceUpdatesMutatedLabel(ctx)).To(MatchError(And(ContainSubstring("kube-system/a"), ContainSubstring("fake patch error"))))
		})

		It("should ignore VPAs that were deleted in the meantime", func() {
			withClient(fakeclient.NewClientBuilder().
				WithScheme(kubernetes.ShootScheme).
				WithObjects(newVPA("a", metav1.NamespaceSystem, map[string]string{mutatedLabel: "true"})).
				WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(_ context.Context, _ client.WithWatch, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
						return apierrors.NewNotFound(schema.GroupResource{Group: "autoscaling.k8s.io", Resource: "verticalpodautoscalers"}, obj.GetName())
					},
				}).
				Build())

			Expect(botanist.RemoveVPAInPlaceUpdatesMutatedLabel(ctx)).To(Succeed())
		})
	})
})
