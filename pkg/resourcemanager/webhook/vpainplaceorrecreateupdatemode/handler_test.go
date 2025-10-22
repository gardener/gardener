// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpainplaceorrecreateupdatemode

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/logger"
)

var _ = Describe("Handler", func() {
	var (
		ctx     = context.TODO()
		log     logr.Logger
		handler *Handler

		vpa *vpaautoscalingv1.VerticalPodAutoscaler
	)

	BeforeEach(func() {
		ctx = admission.NewContextWithRequest(ctx, admission.Request{})
		log = logger.MustNewZapLogger(logger.InfoLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		handler = &Handler{Logger: log}

		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{}
	})

	Describe("#Default", func() {
		It("should modify update mode Auto to InPlaceOrRecreate", func() {
			vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeAuto),
			}

			Expect(handler.Default(ctx, vpa)).To(Succeed())
			Expect(vpa.Spec.UpdatePolicy.UpdateMode).To(Equal(ptr.To(vpaautoscalingv1.UpdateModeInPlaceOrRecreate)))
		})

		It("should modify update mode Recreate to InPlaceOrRecreate", func() {
			vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeRecreate),
			}

			Expect(handler.Default(ctx, vpa)).To(Succeed())
			Expect(vpa.Spec.UpdatePolicy.UpdateMode).To(Equal(ptr.To(vpaautoscalingv1.UpdateModeInPlaceOrRecreate)))
		})

		It("should not modify update mode when it is already set to InPlaceOrRecreate", func() {
			vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeInPlaceOrRecreate),
			}

			Expect(handler.Default(ctx, vpa)).To(Succeed())
			Expect(vpa.Spec.UpdatePolicy.UpdateMode).To(Equal(ptr.To(vpaautoscalingv1.UpdateModeInPlaceOrRecreate)))
		})

		It("should modify update mode when update policy is not set", func() {
			Expect(handler.Default(ctx, vpa)).To(Succeed())
			Expect(vpa.Spec.UpdatePolicy.UpdateMode).To(Equal(ptr.To(vpaautoscalingv1.UpdateModeInPlaceOrRecreate)))
		})

		It("should not modify update mode when it is set to Initial", func() {
			vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeInitial),
			}

			Expect(handler.Default(ctx, vpa)).To(Succeed())
			Expect(vpa.Spec.UpdatePolicy.UpdateMode).To(Equal(ptr.To(vpaautoscalingv1.UpdateModeInitial)))
		})

		It("should not modify update mode when it is set to Off", func() {
			vpa.Spec.UpdatePolicy = &vpaautoscalingv1.PodUpdatePolicy{
				UpdateMode: ptr.To(vpaautoscalingv1.UpdateModeOff),
			}

			Expect(handler.Default(ctx, vpa)).To(Succeed())
			Expect(vpa.Spec.UpdatePolicy.UpdateMode).To(Equal(ptr.To(vpaautoscalingv1.UpdateModeOff)))
		})

	})
})
