// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package crddeployer_test

import (
	"context"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/crddeployer"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/pkg/utils/test"
	"github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("CRD", func() {
	var (
		ctx        context.Context
		applier    kubernetes.Applier
		testClient client.Client

		removeFinalizerOnDeletionAnnotation = func(ctx context.Context, crdName string, c client.Client) {
			ExpectWithOffset(1, retry.Until(ctx, 1*time.Second, func(ctx context.Context) (done bool, err error) {
				currentCRD := &apiextensionsv1.CustomResourceDefinition{}
				if err := c.Get(ctx, client.ObjectKey{Name: crdName}, currentCRD); err != nil {
					return false, err
				}

				if v, ok := currentCRD.Annotations[v1beta1constants.ConfirmationDeletion]; ok && v == "true" {
					currentCRD.Finalizers = slices.DeleteFunc(currentCRD.Finalizers, func(el string) bool {
						return el == "foo"
					})
					if err := c.Update(ctx, currentCRD); err != nil {
						return false, err
					}
					return true, nil
				}

				return false, nil
			})).To(Succeed())
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

		confirmationCRD = &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "myresources.mygroup.example.com",
				Annotations: map[string]string{
					"gardener.cloud/deletion-protected": "true",
				},
				Finalizers: []string{
					"foo",
				},
			},
			Status: apiextensionsv1.CustomResourceDefinitionStatus{
				Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
					{Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue},
					{Type: apiextensionsv1.NamesAccepted, Status: apiextensionsv1.ConditionTrue},
				},
			},
		}

		crd1                  string
		crd2                  string
		confirmationCRDString string
		crd1Ready             string

		crdDeployer component.DeployWaiter
	)

	BeforeEach(func() {
		ctx = context.Background()
		var err error

		crd1 = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
    name: myresources.mygroup.example.com`
		confirmationCRDString = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
    name: myresources.mygroup.example.com
    labels:
      gardener.cloud/deletion-protected: "true"
status:
    conditions:
    - type: NamesAccepted
      status: "True"
    - type: Established
      status: "True"`
		crd2 = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
    name: yourresources.mygroup.example.com`

		crd1Ready = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
    name: myresources.mygroup.example.com
status:
    conditions:
    - type: NamesAccepted
      status: "True"
    - type: Established
      status: "True"`

		testClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
		mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)
		applier = kubernetes.NewApplier(testClient, mapper)
		crdDeployer, err = New(testClient, applier, []string{crd1, crd2}, false)
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

	Describe("#Destroy for CRDs that need deletion confirmation", func() {
		It("should not destroy CRDs when CRDDeployer has confirmDeletion set to false", func() {
			crdDeployer, err := New(testClient, applier, []string{confirmationCRDString}, false)
			Expect(err).NotTo(HaveOccurred())
			Expect(crdDeployer).NotTo(BeNil())

			Expect(testClient.Create(ctx, confirmationCRD.DeepCopy())).To(Succeed())

			go removeFinalizerOnDeletionAnnotation(ctx, confirmationCRD.Name, testClient)

			Eventually(component.OpWait(crdDeployer).Destroy).WithArguments(ctx).Should(MatchError(ContainSubstring("resource /myresources.mygroup.example.com still exists")))
		})

		It("should destroy CRDs when CRDDeployer has confirmDeletion set to true", func() {
			crdDeployer, err := New(testClient, applier, []string{confirmationCRDString}, true)
			Expect(err).NotTo(HaveOccurred())
			Expect(crdDeployer).NotTo(BeNil())

			Expect(testClient.Create(ctx, confirmationCRD.DeepCopy())).To(Succeed())

			go removeFinalizerOnDeletionAnnotation(ctx, confirmationCRD.Name, testClient)

			Eventually(component.OpWait(crdDeployer).Destroy).WithArguments(ctx).Should(Succeed())
		})
	})

	Describe("#Wait", func() {
		It("should return true because the CRD is ready", func() {
			// Use a CRDDeployer that deploys a CRD that already has the ready status
			crdDeployer, err := New(testClient, applier, []string{crd1Ready}, false)
			Expect(err).NotTo(HaveOccurred())

			Expect(crdDeployer.Deploy(ctx)).To(Succeed())

			Expect(crdDeployer.Wait(ctx)).To(Succeed())
		})

		It("should time out because CRD is not ready", func() {
			// lower waiting timeout so that the unit test itself does not time out.
			DeferCleanup(test.WithVar(&kubernetesutils.WaitTimeout, 10*time.Millisecond))

			Expect(crdDeployer.Deploy(ctx)).To(Succeed())

			// This works, because the applied manifests `crd1` and `crd2` don't have their status field set.
			// The testEnvironment API server does not set them, so this `Wait()` fails.
			Expect(crdDeployer.Wait(ctx)).
				To(MatchError(ContainSubstring("retry failed with context deadline exceeded, last error: condition \"NamesAccepted\" is missing")))
		})
	})

	Describe("#WaitCleanup", func() {
		It("should return because the CRD is gone", func() {
			Expect(crdDeployer.Deploy(ctx)).To(Succeed())

			Expect(crdDeployer.Destroy(ctx)).To(Succeed())

			Expect(crdDeployer.WaitCleanup(ctx)).To(Succeed())
		})

		It("should time out because CRD is still present", func() {
			// lower waiting timeout so that the unit test itself does not time out.
			DeferCleanup(test.WithVar(&kubernetesutils.WaitTimeout, 10*time.Millisecond))

			Expect(crdDeployer.Deploy(ctx)).To(Succeed())

			// WaitCleanup fails here intentionally, because the CRDs were deployed, but not cleaned up.
			Expect(crdDeployer.WaitCleanup(ctx)).
				To(MatchError(ContainSubstring("context deadline exceeded")))
		})
	})
})
