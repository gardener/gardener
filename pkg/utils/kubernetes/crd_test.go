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
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("CRD", func() {
	var (
		ctx               context.Context
		testClientBuilder *fakeclient.ClientBuilder
		testClient        client.Client

		unreadyCRD = &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "myresources.mygroup.example.com",
			},
		}
		readyCRD = &apiextensionsv1.CustomResourceDefinition{
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
	)

	BeforeEach(func() {
		// lower waiting timeout so that the unit test itself does not time out.
		DeferCleanup(test.WithVar(&WaitTimeout, 10*time.Millisecond))

		ctx = context.Background()
		testClientBuilder = fakeclient.NewClientBuilder().WithScheme(apiextensionsscheme.Scheme)
	})

	Describe("#WaitUntilCRDManifestsReady", func() {
		It("should return true because the CRD is ready", func() {
			testClient = testClientBuilder.WithObjects(readyCRD).Build()

			Expect(WaitUntilCRDManifestsReady(ctx, testClient, "myresources.mygroup.example.com")).
				To(Succeed())
		})

		It("should time out because CRD is not ready", func() {
			testClient = testClientBuilder.WithObjects(unreadyCRD).Build()

			Expect(WaitUntilCRDManifestsReady(ctx, testClient, "myresources.mygroup.example.com")).
				To(MatchError(ContainSubstring("retry failed with context deadline exceeded, last error: condition \"NamesAccepted\" is missing")))
		})

		It("should time out because CRD is not present", func() {
			// testClient without any objects.
			testClient = testClientBuilder.Build()

			Expect(WaitUntilCRDManifestsReady(ctx, testClient, "myresources.mygroup.example.com")).
				To(MatchError(ContainSubstring("retry failed with context deadline exceeded, last error: customresourcedefinitions.apiextensions.k8s.io \"myresources.mygroup.example.com\" not found")))

		})
	})

	Describe("#WaitUntilManifestsDestroyed", func() {
		It("should return because the CRD is gone", func() {
			testClient = testClientBuilder.WithObjects(readyCRD).Build()

			Expect(testClient.Delete(ctx, readyCRD)).To(Succeed())

			Expect(WaitUntilCRDManifestsDestroyed(ctx, testClient, readyCRD.Name)).To(Succeed())
		})

		It("should time out because CRD is not ready", func() {
			testClient = testClientBuilder.WithObjects(unreadyCRD).Build()

			Expect(WaitUntilCRDManifestsDestroyed(ctx, testClient, readyCRD.Name)).
				To(MatchError(ContainSubstring("context deadline exceeded")))
		})
	})
})
