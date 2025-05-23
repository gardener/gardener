// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/gardener/gardener/extensions/pkg/controller"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

var _ = Describe("Status", func() {
	var (
		ctx     = context.TODO()
		fakeErr = errors.New("fake")

		generation int64 = 1337
		lastOpType       = gardencorev1beta1.LastOperationTypeCreate
		lastOpDesc       = "foo"
		caser            = cases.Title(language.English)

		ctrl *gomock.Controller
		log  logr.Logger
		c    *mockclient.MockClient
		sw   *mockclient.MockStatusWriter

		statusUpdater StatusUpdater
		obj           extensionsv1alpha1.Object
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		c = mockclient.NewMockClient(ctrl)
		sw = mockclient.NewMockStatusWriter(ctrl)

		statusUpdater = NewStatusUpdater(c)

		obj = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Generation: generation,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Processing", func() {
		It("should return an error if the Patch() call fails", func() {
			gomock.InOrder(
				c.EXPECT().Status().Return(sw),
				sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{}), gomock.Any()).Return(fakeErr),
			)

			Expect(statusUpdater.Processing(ctx, log, obj, lastOpType, lastOpDesc)).To(MatchError(fakeErr))
		})

		It("should update the last operation as expected", func() {
			gomock.InOrder(
				c.EXPECT().Status().Return(sw),
				sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{}), gomock.Any()).Do(func(_ context.Context, obj extensionsv1alpha1.Object, _ client.Patch, _ ...client.PatchOption) {
					lastOperation := obj.GetExtensionStatus().GetLastOperation()

					Expect(lastOperation.Type).To(Equal(lastOpType))
					Expect(lastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateProcessing))
					Expect(lastOperation.Progress).To(Equal(int32(1)))
					Expect(lastOperation.Description).To(Equal(lastOpDesc))
				}),
			)

			Expect(statusUpdater.Processing(ctx, log, obj, lastOpType, lastOpDesc)).To(Succeed())
		})
	})

	Describe("#Error", func() {
		It("should return an error if the Patch() call fails", func() {
			gomock.InOrder(
				c.EXPECT().Status().Return(sw),
				sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{}), gomock.Any()).Return(fakeErr),
			)

			Expect(statusUpdater.Error(ctx, log, obj, fakeErr, lastOpType, lastOpDesc)).To(MatchError(fakeErr))
		})

		It("should update the last operation as expected (w/o error codes)", func() {
			gomock.InOrder(
				c.EXPECT().Status().Return(sw),
				sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{}), gomock.Any()).Do(func(_ context.Context, obj extensionsv1alpha1.Object, _ client.Patch, _ ...client.PatchOption) {
					var (
						description = caser.String(lastOpDesc) + ": " + fakeErr.Error()

						lastOperation      = obj.GetExtensionStatus().GetLastOperation()
						lastError          = obj.GetExtensionStatus().GetLastError()
						observedGeneration = obj.GetExtensionStatus().GetObservedGeneration()
					)

					Expect(observedGeneration).To(Equal(generation))

					Expect(lastOperation.Type).To(Equal(lastOpType))
					Expect(lastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
					Expect(lastOperation.Progress).To(Equal(int32(50)))
					Expect(lastOperation.Description).To(Equal(description))

					Expect(lastError.Description).To(Equal(description))
					Expect(lastError.TaskID).To(BeNil())
					Expect(lastError.Codes).To(BeEmpty())
				}),
			)

			Expect(statusUpdater.Error(ctx, log, obj, fakeErr, lastOpType, lastOpDesc)).To(Succeed())
		})

		It("should update the last operation as expected (w/ error codes)", func() {
			err := helper.NewErrorWithCodes(fmt.Errorf("unauthorized"), gardencorev1beta1.ErrorInfraUnauthorized)

			gomock.InOrder(
				c.EXPECT().Status().Return(sw),
				sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{}), gomock.Any()).Do(func(_ context.Context, obj extensionsv1alpha1.Object, _ client.Patch, _ ...client.PatchOption) {
					var (
						description = caser.String(lastOpDesc) + ": " + err.Error()

						lastOperation      = obj.GetExtensionStatus().GetLastOperation()
						lastError          = obj.GetExtensionStatus().GetLastError()
						observedGeneration = obj.GetExtensionStatus().GetObservedGeneration()
					)

					Expect(observedGeneration).To(Equal(generation))

					Expect(lastOperation.Type).To(Equal(lastOpType))
					Expect(lastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
					Expect(lastOperation.Progress).To(Equal(int32(50)))
					Expect(lastOperation.Description).To(Equal(description))

					Expect(lastError.Description).To(Equal(description))
					Expect(lastError.TaskID).To(BeNil())
					Expect(lastError.Codes).To(ConsistOf(gardencorev1beta1.ErrorInfraUnauthorized))
				}),
			)

			Expect(statusUpdater.Error(ctx, log, obj, err, lastOpType, lastOpDesc)).To(Succeed())
		})
	})

	Describe("#Success", func() {
		It("should return an error if the Patch() call fails", func() {
			gomock.InOrder(
				c.EXPECT().Status().Return(sw),
				sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{}), gomock.Any()).Return(fakeErr),
			)

			Expect(statusUpdater.Success(ctx, log, obj, lastOpType, lastOpDesc)).To(MatchError(fakeErr))
		})

		It("should update the last operation as expected", func() {
			gomock.InOrder(
				c.EXPECT().Status().Return(sw),
				sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&extensionsv1alpha1.Infrastructure{}), gomock.Any()).Do(func(_ context.Context, obj extensionsv1alpha1.Object, _ client.Patch, _ ...client.PatchOption) {
					var (
						lastOperation      = obj.GetExtensionStatus().GetLastOperation()
						lastError          = obj.GetExtensionStatus().GetLastError()
						observedGeneration = obj.GetExtensionStatus().GetObservedGeneration()
					)

					Expect(observedGeneration).To(Equal(generation))

					Expect(lastOperation.Type).To(Equal(lastOpType))
					Expect(lastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
					Expect(lastOperation.Progress).To(Equal(int32(100)))
					Expect(lastOperation.Description).To(Equal(lastOpDesc))

					Expect(lastError).To(BeNil())
				}),
			)

			Expect(statusUpdater.Success(ctx, log, obj, lastOpType, lastOpDesc)).To(Succeed())
		})
	})
})
