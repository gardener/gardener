// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package required_test

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	"github.com/gardener/gardener/pkg/logger"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/extension/required"
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
			reconciler = &Reconciler{
				Lock: &sync.RWMutex{},
			}
		})

		Describe("#MapObjectKindToExtensions", func() {
			var (
				fakeClient          client.Client
				kindToRequiredTypes map[string]sets.Set[string]

				mapperFunc mapper.MapFunc
			)

			BeforeEach(func() {
				kindToRequiredTypes = map[string]sets.Set[string]{}
				fakeClient = fake.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()

				reconciler.KindToRequiredTypes = kindToRequiredTypes
				reconciler.Client = fakeClient

				mapperFunc = reconciler.MapObjectKindToExtensions("BackupBucket", func() client.ObjectList { return &extensionsv1alpha1.BackupBucketList{} })
			})

			Context("without extensions", func() {
				It("should not return any requests", func() {
					requests := mapperFunc.Map(ctx, log, nil, nil)
					Expect(requests).To(BeEmpty())
				})
			})

			Context("with extensions", func() {
				var (
					testExtension1, testExtension2 *operatorv1alpha1.Extension

					requiredExtensionKind string
					requiredExtensionType string
				)

				BeforeEach(func() {
					requiredExtensionKind = "BackupBucket"
					requiredExtensionType = "local"

					testExtension1 = &operatorv1alpha1.Extension{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-extension-1",
						},
						Spec: operatorv1alpha1.ExtensionSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{Kind: requiredExtensionKind, Type: requiredExtensionType},
							},
						},
					}

					testExtension2 = &operatorv1alpha1.Extension{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-extension-2",
						},
						Spec: operatorv1alpha1.ExtensionSpec{
							Resources: []gardencorev1beta1.ControllerResource{
								{Kind: "DNSRecord", Type: requiredExtensionType},
							},
						},
					}

					Expect(fakeClient.Create(ctx, testExtension1)).To(Succeed())
					Expect(fakeClient.Create(ctx, testExtension2)).To(Succeed())
				})

				It("should add the kind with an empty set to the map and return the extension", func() {
					requests := mapperFunc.Map(ctx, log, nil, nil)
					Expect(requests).To(ConsistOf(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: testExtension1.Name, Namespace: testExtension1.Namespace}})))
					Expect(kindToRequiredTypes).To(HaveKeyWithValue(requiredExtensionKind, sets.New[string]()))
				})

				It("should correctly calculate the kind-to-types map and return the expected extension in the requests", func() {
					By("Invoke mapper the first time and expect requests")
					backupBucket := &extensionsv1alpha1.BackupBucket{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-backup-bucket",
						},
						Spec: extensionsv1alpha1.BackupBucketSpec{
							DefaultSpec: extensionsv1alpha1.DefaultSpec{
								Type:  requiredExtensionType,
								Class: ptr.To(extensionsv1alpha1.ExtensionClassGarden),
							},
						},
					}

					Expect(fakeClient.Create(ctx, backupBucket)).To(Succeed())

					requests := mapperFunc.Map(ctx, log, nil, nil)
					Expect(requests).To(ConsistOf(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: testExtension1.Name, Namespace: testExtension1.Namespace}})))
					Expect(kindToRequiredTypes).To(HaveKeyWithValue(requiredExtensionKind, sets.New[string](requiredExtensionType)))

					By("Invoke mapper again w/o changes and expect no requests")
					requests = mapperFunc.Map(ctx, log, nil, nil)
					Expect(kindToRequiredTypes).To(HaveKeyWithValue(requiredExtensionKind, sets.New[string](requiredExtensionType)))
					Expect(requests).To(BeEmpty())

					By("Delete BackupBucket and expect the extension in the requests")
					Expect(fakeClient.Delete(ctx, backupBucket)).To(Succeed())
					requests = mapperFunc.Map(ctx, log, nil, nil)
					Expect(requests).To(ConsistOf(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: testExtension1.Name, Namespace: testExtension1.Namespace}})))
					Expect(kindToRequiredTypes).To(HaveKeyWithValue(requiredExtensionKind, sets.New[string]()))
				})
			})
		})
	})
})
