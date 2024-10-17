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
	cli "sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("CRD", func() {
	var (
		ctx context.Context

		UnreadyCrd = &apiextensionsv1.CustomResourceDefinition{}
		ReadyCrd   = &apiextensionsv1.CustomResourceDefinition{
			Status: apiextensionsv1.CustomResourceDefinitionStatus{
				Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
					{Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue},
					{Type: apiextensionsv1.NamesAccepted, Status: apiextensionsv1.ConditionTrue},
				},
			},
		}

		ValidManifest = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
    name: myresources.mygroup.example.com`
		InValidManifest = `thisIsNotAValidManifest`
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("#WaitUntilCRDManifestsReady", func() {
		It("should return true because the CRD is ready", func() {
			fakeClient := fakeclient.NewClientBuilder().
				WithScheme(apiextensionsscheme.Scheme).
				WithObjects(ReadyCrd).
				Build()

			err := WaitUntilCRDManifestsReady(ctx, fakeClient, []cli.ObjectKey{cli.ObjectKeyFromObject(ReadyCrd)})

			Expect(err).ToNot(HaveOccurred())
		})

		It("should time out because CRD is not ready", func() {
			// lower waiting timeout so that the unit test itself does not time out
			CRDWaitTimeout = 100 * time.Millisecond
			fakeClient := fakeclient.NewClientBuilder().
				WithScheme(apiextensionsscheme.Scheme).
				WithObjects(UnreadyCrd).
				Build()

			err := WaitUntilCRDManifestsReady(ctx, fakeClient, []cli.ObjectKey{cli.ObjectKeyFromObject(UnreadyCrd)})

			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("context deadline exceeded")))
		})
	})

	Describe("#GetObjectKeyFromManifest", func() {
		It("should return the correct object key from the manifest", func() {
			objKey, err := GetObjectKeyFromManifest(ValidManifest)

			Expect(err).ToNot(HaveOccurred())
			Expect(objKey).To(Equal(cli.ObjectKey{Name: "myresources.mygroup.example.com"}))
		})

		It("should throw an error if no valid manifest is passed", func() {
			objKey, err := GetObjectKeyFromManifest(InValidManifest)

			Expect(objKey).To(Equal(cli.ObjectKey{}))
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal")))
		})
	})
})
