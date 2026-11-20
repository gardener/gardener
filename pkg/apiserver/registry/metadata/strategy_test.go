// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/apis/core"
)

var _ = Describe("Strategy", func() {
	var (
		ctx = context.Background()

		parent   rest.RESTUpdateStrategy
		s        strategy
		oldShoot *core.Shoot
		newShoot *core.Shoot
	)

	BeforeEach(func() {
		parent = fakeParentStrategy{}
		s = newStrategy(parent)

		oldShoot = &core.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "foo",
				Namespace:   "bar",
				Labels:      map[string]string{"old": "label"},
				Annotations: map[string]string{"old": "annotation"},
				Finalizers:  []string{"old.finalizer"},
			},
			Spec:   core.ShootSpec{Region: "region-old"},
			Status: core.ShootStatus{TechnicalID: "technical-id-old"},
		}

		newShoot = oldShoot.DeepCopy()
	})

	Describe("#PrepareForUpdate", func() {
		It("should apply the incoming metadata", func() {
			newShoot.Labels = map[string]string{"new": "label"}
			newShoot.Annotations = map[string]string{"new": "annotation"}
			newShoot.Finalizers = []string{"new.finalizer"}
			newShoot.OwnerReferences = []metav1.OwnerReference{{Name: "owner"}}
			newShoot.ManagedFields = []metav1.ManagedFieldsEntry{{Manager: "manager"}}

			s.PrepareForUpdate(ctx, newShoot, oldShoot)

			Expect(newShoot.Labels).To(Equal(map[string]string{"new": "label"}))
			Expect(newShoot.Annotations).To(Equal(map[string]string{"new": "annotation"}))
			Expect(newShoot.Finalizers).To(Equal([]string{"new.finalizer"}))
			Expect(newShoot.OwnerReferences).To(Equal([]metav1.OwnerReference{{Name: "owner"}}))
			Expect(newShoot.ManagedFields).To(Equal([]metav1.ManagedFieldsEntry{{Manager: "manager"}}))
		})

		It("should restore the spec from the stored object", func() {
			newShoot.Spec.Region = "region-new"

			s.PrepareForUpdate(ctx, newShoot, oldShoot)

			Expect(newShoot.Spec.Region).To(Equal("region-old"))
		})

		It("should restore the status from the stored object", func() {
			newShoot.Status.TechnicalID = "technical-id-new"

			s.PrepareForUpdate(ctx, newShoot, oldShoot)

			Expect(newShoot.Status.TechnicalID).To(Equal("technical-id-old"))
		})

		It("should preserve the name and namespace", func() {
			s.PrepareForUpdate(ctx, newShoot, oldShoot)

			Expect(newShoot.Name).To(Equal("foo"))
			Expect(newShoot.Namespace).To(Equal("bar"))
		})

		It("should propagate metadata changes the parent strategy performs", func() {
			newShoot.Annotations = map[string]string{"gardener.cloud/operation": "reconcile"}
			parent = fakeParentStrategy{prepareForUpdate: func(_ context.Context, obj, _ runtime.Object) {
				delete(obj.(*core.Shoot).Annotations, "gardener.cloud/operation")
			}}
			s = newStrategy(parent)

			s.PrepareForUpdate(ctx, newShoot, oldShoot)

			Expect(newShoot.Annotations).NotTo(HaveKey("gardener.cloud/operation"))
		})

		It("should carry over a generation increment the parent strategy performs", func() {
			oldShoot.Generation = 5
			newShoot = oldShoot.DeepCopy()
			parent = fakeParentStrategy{prepareForUpdate: func(_ context.Context, obj, old runtime.Object) {
				obj.(*core.Shoot).Generation = old.(*core.Shoot).Generation + 1
			}}
			s = newStrategy(parent)

			s.PrepareForUpdate(ctx, newShoot, oldShoot)

			Expect(newShoot.Generation).To(Equal(int64(6)))
		})

		It("should discard spec changes the parent strategy performs", func() {
			parent = fakeParentStrategy{prepareForUpdate: func(_ context.Context, obj, _ runtime.Object) {
				obj.(*core.Shoot).Spec.Region = "region-from-parent"
			}}
			s = newStrategy(parent)

			s.PrepareForUpdate(ctx, newShoot, oldShoot)

			Expect(newShoot.Spec.Region).To(Equal("region-old"))
		})

		It("should discard status changes the parent strategy performs", func() {
			parent = fakeParentStrategy{prepareForUpdate: func(_ context.Context, obj, _ runtime.Object) {
				obj.(*core.Shoot).Status.TechnicalID = "technical-id-from-parent"
			}}
			s = newStrategy(parent)

			s.PrepareForUpdate(ctx, newShoot, oldShoot)

			Expect(newShoot.Status.TechnicalID).To(Equal("technical-id-old"))
		})

		It("should not allow the request to change the generation", func() {
			oldShoot.Generation = 5
			newShoot = oldShoot.DeepCopy()
			newShoot.Generation = 99

			s.PrepareForUpdate(ctx, newShoot, oldShoot)

			Expect(newShoot.Generation).To(Equal(int64(5)))
		})
	})

	Describe("#ValidateUpdate", func() {
		It("should not return any errors", func() {
			Expect(s.ValidateUpdate(ctx, newShoot, oldShoot)).To(BeEmpty())
		})
	})

	Describe("#WarningsOnUpdate", func() {
		It("should not return any warnings", func() {
			Expect(s.WarningsOnUpdate(ctx, newShoot, oldShoot)).To(BeEmpty())
		})
	})
})

type fakeParentStrategy struct {
	rest.RESTUpdateStrategy

	prepareForUpdate func(context.Context, runtime.Object, runtime.Object)
}

func (fakeParentStrategy) NamespaceScoped() bool { return true }

func (f fakeParentStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	if f.prepareForUpdate != nil {
		f.prepareForUpdate(ctx, obj, old)
	}
}

func (fakeParentStrategy) ValidateUpdate(context.Context, runtime.Object, runtime.Object) field.ErrorList {
	return nil
}
