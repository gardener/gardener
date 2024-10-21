// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("CRD", func() {
	var (
		ctx context.Context

		UnreadyCrd = &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "myresources.mygroup.example.com",
			},
		}
		ReadyCrd = &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "myresources.mygroup.example.com",
			},
			Status: apiextensionsv1.CustomResourceDefinitionStatus{
				Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
					{Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue},
					{Type: apiextensionsv1.NamesAccepted, Status: apiextensionsv1.ConditionTrue},
				},
			},
		}

		ValidManifest   string
		InvalidManifest string
	)

	BeforeEach(func() {
		ctx = context.Background()
		ValidManifest = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
    name: myresources.mygroup.example.com`
		InvalidManifest = `thisIsNotAValidManifest`
	})

	Describe("#MakeCrdNameMap", func() {
		It("should return a map representing the CRD name map", func() {
			crdNameToManifest := MakeCrdNameMap([]string{ValidManifest})
			Expect(crdNameToManifest).To(HaveKeyWithValue("myresources.mygroup.example.com", ValidManifest))
		})

		It("should panic when a non valid CRD is provided", func() {
			Expect(func() { MakeCrdNameMap([]string{InvalidManifest}) }).To(Panic())
		})
	})

	Describe("#WaitUntilCRDManifestsReady", func() {
		It("should return true because the CRD is ready", func() {
			fakeClient := fakeclient.NewClientBuilder().
				WithScheme(apiextensionsscheme.Scheme).
				WithObjects(ReadyCrd).
				Build()

			err := WaitUntilCRDManifestsReady(ctx, fakeClient, []string{"myresources.mygroup.example.com"})

			Expect(err).ToNot(HaveOccurred())
		})

		It("should time out because CRD is not ready", func() {
			// lower waiting timeout so that the unit test itself does not time out
			CRDWaitTimeout = 100 * time.Millisecond
			fakeClient := fakeclient.NewClientBuilder().
				WithScheme(apiextensionsscheme.Scheme).
				WithObjects(UnreadyCrd).
				Build()

			err := WaitUntilCRDManifestsReady(ctx, fakeClient, []string{"myresources.mygroup.example.com"})

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("context deadline exceeded")))
		})
	})

	Describe("#GetObjectNameFromManifest", func() {
		It("should return the correct object key from the manifest", func() {
			objKey, err := GetObjectNameFromManifest(ValidManifest)

			Expect(err).ToNot(HaveOccurred())
			Expect(objKey).To(Equal("myresources.mygroup.example.com"))
		})

		It("should throw an error if no valid manifest is passed", func() {
			objKey, err := GetObjectNameFromManifest(InvalidManifest)

			Expect(objKey).To(Equal(""))
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal")))
		})
	})
})
