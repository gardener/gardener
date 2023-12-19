// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package kubeapiserverexposure_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/kubeapiserverexposure"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("#InternalNameService", func() {
	var (
		ctx context.Context
		c   client.Client

		serviceObjKey   client.ObjectKey
		defaultDeployer component.Deployer
		namespace       string
		expected        *corev1.Service
	)

	BeforeEach(func() {
		ctx = context.TODO()

		s := runtime.NewScheme()
		Expect(corev1.AddToScheme(s)).To(Succeed())
		c = fake.NewClientBuilder().WithScheme(s).Build()

		namespace = "foobar"
		serviceObjKey = client.ObjectKey{Name: "kube-apiserver", Namespace: namespace}
		expected = &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
			},
			Spec: corev1.ServiceSpec{
				Type:         corev1.ServiceTypeExternalName,
				ExternalName: "kubernetes.default.svc.cluster.local",
			},
		}
	})

	JustBeforeEach(func() {
		defaultDeployer = NewInternalNameService(
			c,
			namespace,
		)
	})

	Context("Deploy", func() {
		It("should create the expected service", func() {
			Expect(defaultDeployer.Deploy(ctx)).To(Succeed())

			actual := &corev1.Service{}
			Expect(c.Get(ctx, serviceObjKey, actual)).To(Succeed())
			Expect(actual.Annotations).To(DeepEqual(expected.Annotations))
			Expect(actual.Labels).To(DeepEqual(expected.Labels))
			Expect(actual.Spec).To(DeepEqual(expected.Spec))
		})
	})

	Context("Destroy", func() {
		It("should delete the service object", func() {
			Expect(c.Create(ctx, expected)).To(Succeed())
			Expect(c.Get(ctx, serviceObjKey, &corev1.Service{})).To(Succeed())

			Expect(defaultDeployer.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, serviceObjKey, &corev1.Service{})).To(BeNotFoundError())
		})
	})
})
