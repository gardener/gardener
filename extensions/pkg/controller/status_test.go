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
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
)

var _ = Describe("Status", func() {
	var (
		ctx     = context.TODO()
		fakeErr = errors.New("fake")

		generation int64 = 1337
		lastOpType       = gardencorev1beta1.LastOperationTypeCreate
		lastOpDesc       = "foo"
		caser            = cases.Title(language.English)

		log logr.Logger
		c   client.Client

		scheme *runtime.Scheme

		statusUpdater StatusUpdater
		obj           extensionsv1alpha1.Object
	)

	BeforeEach(func() {
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))

		scheme = runtime.NewScheme()
		Expect(extensionsv1alpha1.AddToScheme(scheme)).To(Succeed())

		c = fakeclient.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&extensionsv1alpha1.Infrastructure{}).Build()

		statusUpdater = NewStatusUpdater(c)

		obj = &extensionsv1alpha1.Infrastructure{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-infra",
				Namespace:  "test-ns",
				Generation: generation,
			},
		}

		Expect(c.Create(ctx, obj.DeepCopyObject().(client.Object))).To(Succeed())
	})

	Describe("#Processing", func() {
		It("should return an error if the Patch() call fails", func() {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&extensionsv1alpha1.Infrastructure{}).WithInterceptorFuncs(interceptor.Funcs{
				SubResourcePatch: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
					return fakeErr
				},
			}).Build()
			Expect(fakeClient.Create(ctx, obj.DeepCopyObject().(client.Object))).To(Succeed())

			su := NewStatusUpdater(fakeClient)
			Expect(su.Processing(ctx, log, obj, lastOpType, lastOpDesc)).To(MatchError(fakeErr))
		})

		It("should update the last operation as expected", func() {
			Expect(statusUpdater.Processing(ctx, log, obj, lastOpType, lastOpDesc)).To(Succeed())

			updated := &extensionsv1alpha1.Infrastructure{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(obj), updated)).To(Succeed())

			lastOperation := updated.GetExtensionStatus().GetLastOperation()
			Expect(lastOperation.Type).To(Equal(lastOpType))
			Expect(lastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateProcessing))
			Expect(lastOperation.Progress).To(Equal(int32(1)))
			Expect(lastOperation.Description).To(Equal(lastOpDesc))
		})
	})

	Describe("#Error", func() {
		It("should return an error if the Patch() call fails", func() {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&extensionsv1alpha1.Infrastructure{}).WithInterceptorFuncs(interceptor.Funcs{
				SubResourcePatch: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
					return fakeErr
				},
			}).Build()
			Expect(fakeClient.Create(ctx, obj.DeepCopyObject().(client.Object))).To(Succeed())

			su := NewStatusUpdater(fakeClient)
			Expect(su.Error(ctx, log, obj, fakeErr, lastOpType, lastOpDesc)).To(MatchError(fakeErr))
		})

		It("should update the last operation as expected (w/o error codes)", func() {
			Expect(statusUpdater.Error(ctx, log, obj, fakeErr, lastOpType, lastOpDesc)).To(Succeed())

			updated := &extensionsv1alpha1.Infrastructure{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(obj), updated)).To(Succeed())

			description := caser.String(lastOpDesc) + ": " + fakeErr.Error()

			lastOperation := updated.GetExtensionStatus().GetLastOperation()
			lastError := updated.GetExtensionStatus().GetLastError()
			observedGeneration := updated.GetExtensionStatus().GetObservedGeneration()

			Expect(observedGeneration).To(Equal(generation))

			Expect(lastOperation.Type).To(Equal(lastOpType))
			Expect(lastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
			Expect(lastOperation.Progress).To(Equal(int32(50)))
			Expect(lastOperation.Description).To(Equal(description))

			Expect(lastError.Description).To(Equal(description))
			Expect(lastError.TaskID).To(BeNil())
			Expect(lastError.Codes).To(BeEmpty())
		})

		It("should update the last operation as expected (w/ error codes)", func() {
			err := helper.NewErrorWithCodes(fmt.Errorf("unauthorized"), gardencorev1beta1.ErrorInfraUnauthorized)

			Expect(statusUpdater.Error(ctx, log, obj, err, lastOpType, lastOpDesc)).To(Succeed())

			updated := &extensionsv1alpha1.Infrastructure{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(obj), updated)).To(Succeed())

			description := caser.String(lastOpDesc) + ": " + err.Error()

			lastOperation := updated.GetExtensionStatus().GetLastOperation()
			lastError := updated.GetExtensionStatus().GetLastError()
			observedGeneration := updated.GetExtensionStatus().GetObservedGeneration()

			Expect(observedGeneration).To(Equal(generation))

			Expect(lastOperation.Type).To(Equal(lastOpType))
			Expect(lastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
			Expect(lastOperation.Progress).To(Equal(int32(50)))
			Expect(lastOperation.Description).To(Equal(description))

			Expect(lastError.Description).To(Equal(description))
			Expect(lastError.TaskID).To(BeNil())
			Expect(lastError.Codes).To(ConsistOf(gardencorev1beta1.ErrorInfraUnauthorized))
		})
	})

	Describe("#Success", func() {
		It("should return an error if the Patch() call fails", func() {
			fakeClient := fakeclient.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&extensionsv1alpha1.Infrastructure{}).WithInterceptorFuncs(interceptor.Funcs{
				SubResourcePatch: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
					return fakeErr
				},
			}).Build()
			Expect(fakeClient.Create(ctx, obj.DeepCopyObject().(client.Object))).To(Succeed())

			su := NewStatusUpdater(fakeClient)
			Expect(su.Success(ctx, log, obj, lastOpType, lastOpDesc)).To(MatchError(fakeErr))
		})

		It("should update the last operation as expected", func() {
			Expect(statusUpdater.Success(ctx, log, obj, lastOpType, lastOpDesc)).To(Succeed())

			updated := &extensionsv1alpha1.Infrastructure{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(obj), updated)).To(Succeed())

			lastOperation := updated.GetExtensionStatus().GetLastOperation()
			lastError := updated.GetExtensionStatus().GetLastError()
			observedGeneration := updated.GetExtensionStatus().GetObservedGeneration()

			Expect(observedGeneration).To(Equal(generation))

			Expect(lastOperation.Type).To(Equal(lastOpType))
			Expect(lastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
			Expect(lastOperation.Progress).To(Equal(int32(100)))
			Expect(lastOperation.Description).To(Equal(lastOpDesc))

			Expect(lastError).To(BeNil())
		})
	})
})
