// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
)

var _ = Describe("filesystemSecretsManagerClient", func() {
	var (
		ctx = context.Background()
		fs  afero.Afero
		c   client.Client
	)

	BeforeEach(func() {
		fs = afero.Afero{Fs: afero.NewMemMapFs()}
		c = NewFilesystemSecretsManagerClient(fs)
	})

	secret := func(namespace, name string, secretLabels map[string]string, data map[string][]byte) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    secretLabels,
			},
			Data: data,
		}
	}

	Describe("#Create and #Get", func() {
		It("should write a secret and read it back", func() {
			s := secret("kube-system", "my-secret", map[string]string{"foo": "bar"}, map[string][]byte{"key": []byte("value")})
			Expect(c.Create(ctx, s)).To(Succeed())

			got := &corev1.Secret{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(s), got)).To(Succeed())
			Expect(got.Name).To(Equal("my-secret"))
			Expect(got.Namespace).To(Equal("kube-system"))
			Expect(got.Labels).To(Equal(map[string]string{"foo": "bar"}))
			Expect(got.Data).To(Equal(map[string][]byte{"key": []byte("value")}))
		})

		It("should return NotFound for a missing secret", func() {
			missing := secret("kube-system", "missing", nil, nil)
			Expect(apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(missing), &corev1.Secret{}))).To(BeTrue())
		})

		It("should return AlreadyExists on duplicate Create", func() {
			s := secret("kube-system", "dup", nil, nil)
			Expect(c.Create(ctx, s)).To(Succeed())
			Expect(apierrors.IsAlreadyExists(c.Create(ctx, s))).To(BeTrue())
		})
	})

	Describe("#List", func() {
		It("should list secrets filtered by namespace and labels", func() {
			Expect(c.Create(ctx, secret("kube-system", "a", map[string]string{"managed-by": "secrets-manager"}, nil))).To(Succeed())
			Expect(c.Create(ctx, secret("kube-system", "b", map[string]string{"managed-by": "other"}, nil))).To(Succeed())
			Expect(c.Create(ctx, secret("default", "c", map[string]string{"managed-by": "secrets-manager"}, nil))).To(Succeed())

			list := &corev1.SecretList{}
			Expect(c.List(ctx, list,
				client.InNamespace("kube-system"),
				client.MatchingLabels{"managed-by": "secrets-manager"},
			)).To(Succeed())
			Expect(list.Items).To(HaveLen(1))
			Expect(list.Items[0].Name).To(Equal("a"))
		})

		It("should return empty list when directory does not exist", func() {
			list := &corev1.SecretList{}
			Expect(c.List(ctx, list)).To(Succeed())
			Expect(list.Items).To(BeEmpty())
		})
	})

	Describe("#Patch", func() {
		It("should apply a merge patch updating labels", func() {
			s := secret("kube-system", "patched", map[string]string{"old": "label"}, nil)
			Expect(c.Create(ctx, s)).To(Succeed())

			updated := s.DeepCopy()
			updated.Labels = map[string]string{"new": "label"}
			patch := client.MergeFrom(s)

			Expect(c.Patch(ctx, updated, patch)).To(Succeed())

			got := &corev1.Secret{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(updated), got)).To(Succeed())
			Expect(got.Labels).To(Equal(map[string]string{"new": "label"}))
		})

		It("should return NotFound when patching a non-existent secret", func() {
			s := secret("kube-system", "ghost", nil, nil)
			existing := s.DeepCopy()
			patch := client.MergeFrom(existing)
			Expect(apierrors.IsNotFound(c.Patch(ctx, s, patch))).To(BeTrue())
		})
	})

	Describe("#Delete", func() {
		It("should delete an existing secret", func() {
			s := secret("kube-system", "todel", nil, nil)
			Expect(c.Create(ctx, s)).To(Succeed())
			Expect(c.Delete(ctx, s)).To(Succeed())

			got := &corev1.Secret{}
			Expect(apierrors.IsNotFound(c.Get(ctx, client.ObjectKeyFromObject(s), got))).To(BeTrue())
		})

		It("should return NotFound when deleting a non-existent secret", func() {
			s := secret("kube-system", "ghost", nil, nil)
			Expect(apierrors.IsNotFound(c.Delete(ctx, s))).To(BeTrue())
		})
	})
})
