// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/etcd/bootstrap"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Etcd", func() {
	var (
		c           client.Client
		sm          secretsmanager.Interface
		etcd        component.Deployer
		ctx         = context.Background()
		namespace   = "shoot--foo--bar"
		image       = "some-image"
		statefulSet = &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "etcd-main-0",
				Namespace: namespace,
			},
		}
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(c, namespace)
		etcd = New(c, namespace, sm, Values{Image: image, Role: "main"})

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-etcd-peer", Namespace: namespace}})).To(Succeed())
	})

	Describe("#Deploy", func() {
		It("should successfully deploy bootstrap etcd", func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(statefulSet), statefulSet)).To(BeNotFoundError())
			Expect(etcd.Deploy(ctx)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(statefulSet), statefulSet)).To(Succeed())
			Expect(statefulSet.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(statefulSet.Spec.Template.Spec.Containers[0].Image).To(Equal(image))
			Expect(statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts).To(HaveLen(6))
			Expect(statefulSet.Spec.Template.Spec.Containers[0].VolumeMounts).Should(ContainElements([]gomegatypes.GomegaMatcher{
				MatchFields(IgnoreExtras, Fields{"Name": Equal("data"), "MountPath": Equal("/var/etcd/data")}),
				MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-ca"), "MountPath": Equal("/var/etcd/ssl/ca")}),
				MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-server-tls"), "MountPath": Equal("/var/etcd/ssl/server")}),
				MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-client-tls"), "MountPath": Equal("/var/etcd/ssl/client")}),
				MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-peer-ca"), "MountPath": Equal("/var/etcd/ssl/peer/ca")}),
				MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-peer-server-tls"), "MountPath": Equal("/var/etcd/ssl/peer/server")}),
			}))
			Expect(statefulSet.Spec.Template.Spec.Volumes).To(HaveLen(6))
			Expect(statefulSet.Spec.Template.Spec.Volumes).Should(ContainElements([]gomegatypes.GomegaMatcher{
				MatchFields(IgnoreExtras, Fields{"Name": Equal("data")}),
				MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-ca")}),
				MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-server-tls")}),
				MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-client-tls")}),
				MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-peer-ca")}),
				MatchFields(IgnoreExtras, Fields{"Name": Equal("etcd-peer-server-tls")}),
			}))
		})
	})

	Describe("#Destroy", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(etcd.Destroy(ctx)).To(Succeed())
		})
	})
})
