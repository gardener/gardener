// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package runtime_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/extension/required/runtime"
)

var _ = Describe("Add", func() {
	Describe("Reconciler", func() {
		var (
			ctx        context.Context
			log        logr.Logger
			reconciler *Reconciler
		)

		BeforeEach(func() {
			ctx = context.Background()
			log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
			reconciler = &Reconciler{}
		})

		Describe("#MapGardenToExtensions", func() {
			var (
				fakeClient client.Client
				garden     *operatorv1alpha1.Garden
				mapperFunc handler.MapFunc
			)

			BeforeEach(func() {
				fakeClient = fake.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
				reconciler.Client = fakeClient

				garden = &operatorv1alpha1.Garden{
					Spec: operatorv1alpha1.GardenSpec{
						DNS: &operatorv1alpha1.DNSManagement{
							Providers: []operatorv1alpha1.DNSProvider{
								{Type: "local-dns"},
							},
						},
						Extensions: []operatorv1alpha1.GardenExtension{
							{Type: "local-extension-1"},
							{Type: "local-extension-2"},
						},
						VirtualCluster: operatorv1alpha1.VirtualCluster{
							ETCD: &operatorv1alpha1.ETCD{
								Main: &operatorv1alpha1.ETCDMain{
									Backup: &operatorv1alpha1.Backup{
										Provider: "local-infrastructure",
									},
								},
							},
						},
					},
				}

				mapperFunc = reconciler.MapGardenToExtensions(log)
			})

			Context("without extensions", func() {
				It("should not return any requests", func() {
					Expect(mapperFunc(ctx, garden)).To(BeEmpty())
				})
			})

			Context("with extensions", func() {
				var (
					infraExtension, dnsExtension *operatorv1alpha1.Extension
				)

				BeforeEach(func() {
					infraExtension = &operatorv1alpha1.Extension{
						ObjectMeta: metav1.ObjectMeta{
							Name: "local-infra",
						},
						Spec: operatorv1alpha1.ExtensionSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{Kind: "BackupBucket", Type: "local-infrastructure"},
							},
						},
					}

					dnsExtension = &operatorv1alpha1.Extension{
						ObjectMeta: metav1.ObjectMeta{
							Name: "local-dns",
						},
						Spec: operatorv1alpha1.ExtensionSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{Kind: "DNSRecord", Type: "local-dns"},
							},
						},
					}

					Expect(fakeClient.Create(ctx, infraExtension)).To(Succeed())
					Expect(fakeClient.Create(ctx, dnsExtension)).To(Succeed())
				})

				It("should return the expected extensions", func() {
					Expect(mapperFunc(ctx, garden)).To(ConsistOf(
						Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: infraExtension.Name}}),
						Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: dnsExtension.Name}}),
					))
				})
			})
		})

		Describe("#MapExtensionToExtensions", func() {
			var (
				fakeClient client.Client
				ext        *extensionsv1alpha1.Extension
				mapperFunc handler.MapFunc
			)

			BeforeEach(func() {
				fakeClient = fake.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
				reconciler.Client = fakeClient

				ext = &extensionsv1alpha1.Extension{
					Spec: extensionsv1alpha1.ExtensionSpec{
						DefaultSpec: extensionsv1alpha1.DefaultSpec{
							Type: "shoot-foo-service",
						},
					},
				}

				mapperFunc = reconciler.MapExtensionToExtensions(log)
			})

			Context("without extensions", func() {
				It("should not return any requests", func() {
					Expect(mapperFunc(ctx, ext)).To(BeEmpty())
				})
			})

			Context("with extensions", func() {
				var (
					fooExtension *operatorv1alpha1.Extension
				)

				BeforeEach(func() {
					fooExtension = &operatorv1alpha1.Extension{
						ObjectMeta: metav1.ObjectMeta{
							Name: "extension-shoot-foo-service",
						},
						Spec: operatorv1alpha1.ExtensionSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{Kind: "Extension", Type: "shoot-foo-service"},
							},
						},
					}

					Expect(fakeClient.Create(ctx, fooExtension)).To(Succeed())
				})

				It("should return the expected extensions", func() {
					Expect(mapperFunc(ctx, ext)).To(ConsistOf(
						Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: fooExtension.Name}}),
					))
				})
			})
		})
	})
})
