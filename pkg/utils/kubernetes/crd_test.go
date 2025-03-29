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
	Describe("WaitUntilCondition", func() {
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

	Describe("#DecodeCRD", func() {
		var (
			crdYAML string
		)
		It("should return error because the CRD is invalid", func() {
			crdYAML = `
apiVersion: apiextensions.k8s.io/v1
kind CustomResourceDefinition
spec:`
			_, err := DecodeCRD(crdYAML)
			Expect(err).To(HaveOccurred())
		})

		It("should successfully decode the CRD", func() {
			// The following valid sample CRD is taken from https://github.com/kubernetes/sample-controller/blob/master/artifacts/examples/crd.yaml
			crdYAML = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: foos.samplecontroller.k8s.io
# for more information on the below annotation, please see
# https://github.com/kubernetes/enhancements/blob/master/keps/sig-api-machinery/2337-k8s.io-group-protection/README.md
  annotations:
    "api-approved.kubernetes.io": "unapproved, experimental-only; please get an approval from Kubernetes API reviewers if you're trying to develop a CRD in the *.k8s.io or *.kubernetes.io groups"
spec:
  group: samplecontroller.k8s.io
  versions:
  - name: v1alpha1
    served: true
    storage: true
    schema:
      # schema used for validation
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              deploymentName:
                type: string
              replicas:
                type: integer
                minimum: 1
                maximum: 10
          status:
            type: object
            properties:
              availableReplicas:
                type: integer
  names:
    kind: Foo
    plural: foos
  scope: Namespaced`
			crdObj, err := DecodeCRD(crdYAML)
			Expect(err).ToNot(HaveOccurred())
			Expect(crdObj).ToNot(BeNil())
		})
	})
})
