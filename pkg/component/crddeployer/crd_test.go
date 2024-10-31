// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crddeployer_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/crddeployer"
	"github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CRD", func() {
	var (
		ctx        context.Context
		applier    kubernetes.Applier
		testClient client.Client

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

		validManifest   string
		invalidManifest string

		crdDeployer component.DeployWaiter
	)

	BeforeEach(func() {
		ctx = context.Background()
		validManifest = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
    name: myresources.mygroup.example.com`
		invalidManifest = `thisIsNotAValidManifest`

		testClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
		mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)
		applier = kubernetes.NewApplier(testClient, mapper)
		var err error
		crdDeployer, err = NewCRDDeployer(testClient, applier, []string{validManifest})
		Expect(err).NotTo(HaveOccurred())
		Expect(crdDeployer).ToNot(BeNil())
	})

	Describe("#Deploy", func() {
		It("should deploy a CRD", func() {
			actualCRD := &apiextensionsv1.CustomResourceDefinition{}

			Expect(crdDeployer.Deploy(ctx)).To(Succeed())

			Expect(testClient.Get(ctx, client.ObjectKey{Name: readyCRD.Name}, actualCRD)).To(Succeed())
			Expect(actualCRD.Name).To(Equal(readyCRD.Name))
		})
	})

	Describe("#Destroy", func() {
		It("should destroy a CRD", func() {
			actualCRD := &apiextensionsv1.CustomResourceDefinition{}

			Expect(testClient.Create(ctx, readyCRD)).To(Succeed())

			Expect(crdDeployer.Destroy(ctx)).To(Succeed())

			Expect(testClient.Get(ctx, client.ObjectKey{Name: readyCRD.Name}, actualCRD)).To(matchers.BeNotFoundError())
		})
	})

	Describe("#MakeCRDNameMap", func() {
		It("should return a map representing the CRD name map", func() {
			crdNameToManifest, err := MakeCRDNameMap([]string{validManifest})
			Expect(err).NotTo(HaveOccurred())
			Expect(crdNameToManifest).To(HaveKeyWithValue("myresources.mygroup.example.com", validManifest))
		})

		It("should throw an error when a non valid CRD is provided", func() {
			crdMap, err := MakeCRDNameMap([]string{invalidManifest})
			Expect(crdMap).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("error unmarshaling JSON")))
		})
	})

	Describe("#WaitUntilCRDManifestsReady", func() {
		It("should return true because the CRD is ready", func() {
			testClient := fakeclient.NewClientBuilder().
				WithScheme(apiextensionsscheme.Scheme).
				WithObjects(readyCRD).
				Build()

			Expect(WaitUntilCRDManifestsReady(ctx, testClient, []string{"myresources.mygroup.example.com"})).
				To(Succeed())
		})

		It("should time out because CRD is not ready", func() {
			// lower waiting timeout so that the unit test itself does not time out
			CRDWaitTimeout = 10 * time.Millisecond
			testClient := fakeclient.NewClientBuilder().
				WithScheme(apiextensionsscheme.Scheme).
				WithObjects(unreadyCRD).
				Build()

			Expect(WaitUntilCRDManifestsReady(ctx, testClient, []string{"myresources.mygroup.example.com"})).
				To(MatchError(ContainSubstring("retry failed with context deadline exceeded, last error: condition \"NamesAccepted\" is missing")))

		})
	})

	Describe("#WaitUntilManifestsDestroyed", func() {
		It("should return because the CRD is gone", func() {
			testClient := fakeclient.NewClientBuilder().
				WithScheme(apiextensionsscheme.Scheme).
				WithObjects(readyCRD).
				Build()

			Expect(testClient.Delete(ctx, readyCRD)).To(Succeed())

			Expect(WaitUntilCRDManifestsDestroyed(ctx, testClient, []string{readyCRD.Name})).To(Succeed())
		})

		It("should time out because CRD is not ready", func() {
			// lower waiting timeout so that the unit test itself does not time out
			CRDWaitTimeout = 10 * time.Millisecond

			Expect(testClient.Create(ctx, unreadyCRD)).To(Succeed())

			Expect(WaitUntilCRDManifestsDestroyed(ctx, testClient, []string{readyCRD.Name})).
				To(MatchError(ContainSubstring("context deadline exceeded")))
		})
	})

	Describe("#GetObjectNameFromManifest", func() {
		It("should return the correct object key from the manifest", func() {
			objKey, err := GetObjectNameFromManifest(validManifest)

			Expect(err).ToNot(HaveOccurred())
			Expect(objKey).To(Equal("myresources.mygroup.example.com"))
		})

		It("should throw an error if no valid manifest is passed", func() {
			objKey, err := GetObjectNameFromManifest(invalidManifest)

			Expect(objKey).To(Equal(""))
			Expect(err).To(MatchError(ContainSubstring("cannot unmarshal")))
		})
	})
})
