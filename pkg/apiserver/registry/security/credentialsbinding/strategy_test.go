// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package credentialsbinding_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/gardener/gardener/pkg/apis/security"
	credentialsbindingregistry "github.com/gardener/gardener/pkg/apiserver/registry/security/credentialsbinding"
)

var _ = Describe("Strategy", func() {
	var credentialsBinding *security.CredentialsBinding

	BeforeEach(func() {
		credentialsBinding = &security.CredentialsBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "profile",
				Namespace: "garden",
			},
			CredentialsRef: corev1.ObjectReference{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "my-secret",
				Namespace:  "my-namespace",
			},
		}
	})

	Describe("#PrepareForCreate", func() {
		It("should set the name if not set", func() {
			credentialsBinding.SetName("")

			credentialsbindingregistry.Strategy.PrepareForCreate(context.TODO(), credentialsBinding)

			Expect(credentialsBinding.GetName()).NotTo(BeEmpty())
		})

		It("should set name with generateName as prefix", func() {
			genName := "prefix-"
			credentialsBinding.GenerateName = genName
			credentialsBinding.Name = ""

			credentialsbindingregistry.Strategy.PrepareForCreate(context.TODO(), credentialsBinding)

			Expect(credentialsBinding.GetGenerateName()).To(Equal(genName))
			Expect(credentialsBinding.GetName()).To(HavePrefix(genName))
		})

		It("should not overwrite already set name", func() {
			credentialsBinding.SetName("bar")

			credentialsbindingregistry.Strategy.PrepareForCreate(context.TODO(), credentialsBinding)

			Expect(credentialsBinding.GetName()).To(Equal("bar"))
		})
	})

	Describe("#ToSelectableFields", func() {
		It("should return the credentialsRef name, namespace and kind", func() {
			result := credentialsbindingregistry.ToSelectableFields(credentialsBinding)

			Expect(result.Get(security.CredentialsBindingCredentialsRefName)).To(Equal("my-secret"))
			Expect(result.Get(security.CredentialsBindingCredentialsRefNamespace)).To(Equal("my-namespace"))
			Expect(result.Get(security.CredentialsBindingCredentialsRefKind)).To(Equal("Secret"))
			Expect(result.Get("metadata.name")).To(Equal("profile"))
			Expect(result.Get("metadata.namespace")).To(Equal("garden"))
		})
	})

	Describe("#GetAttrs", func() {
		It("should return an error when the object is not a CredentialsBinding", func() {
			_, _, err := credentialsbindingregistry.GetAttrs(&security.WorkloadIdentity{})
			Expect(err).To(HaveOccurred())
		})

		It("should return the selectable fields", func() {
			_, fs, err := credentialsbindingregistry.GetAttrs(credentialsBinding)
			Expect(err).NotTo(HaveOccurred())
			Expect(fs.Get(security.CredentialsBindingCredentialsRefName)).To(Equal("my-secret"))
			Expect(fs.Get(security.CredentialsBindingCredentialsRefNamespace)).To(Equal("my-namespace"))
			Expect(fs.Get(security.CredentialsBindingCredentialsRefKind)).To(Equal("Secret"))
		})
	})

	Describe("#MatchCredentialsBinding", func() {
		It("should return the predicate with the given selectors", func() {
			ls := labels.SelectorFromSet(labels.Set{"foo": "bar"})
			fs := fields.OneTermEqualSelector(security.CredentialsBindingCredentialsRefName, "my-secret")

			result := credentialsbindingregistry.MatchCredentialsBinding(ls, fs)

			Expect(result.Label).To(Equal(ls))
			Expect(result.Field).To(Equal(fs))
		})
	})
})
