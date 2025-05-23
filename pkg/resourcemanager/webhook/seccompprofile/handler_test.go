// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seccompprofile_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/resourcemanager/webhook/seccompprofile"
)

var _ = Describe("Handler", func() {
	var (
		ctx     = context.TODO()
		log     logr.Logger
		handler *Handler

		pod *corev1.Pod
	)

	BeforeEach(func() {
		ctx = admission.NewContextWithRequest(ctx, admission.Request{})
		log = logger.MustNewZapLogger(logger.InfoLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		handler = &Handler{Logger: log}

		pod = &corev1.Pod{}
	})

	Describe("#Default", func() {
		It("should not patch the seccomp profile name when the pod already specifies seccomp profile", func() {
			pod.Spec.SecurityContext = &corev1.PodSecurityContext{
				SeccompProfile: &corev1.SeccompProfile{
					Type: corev1.SeccompProfileTypeUnconfined,
				},
			}

			Expect(handler.Default(ctx, pod)).To(Succeed())
			Expect(pod.Spec.SecurityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeUnconfined))
		})

		It("should default the seccomp profile type when it is not explicitly specified", func() {
			Expect(handler.Default(ctx, pod)).To(Succeed())
			Expect(pod.Spec.SecurityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
		})

		It("should default the seccomp profile type when it is not explicitly specified", func() {
			pod.Spec.SecurityContext = &corev1.PodSecurityContext{}

			Expect(handler.Default(ctx, pod)).To(Succeed())
			Expect(pod.Spec.SecurityContext.SeccompProfile.Type).To(Equal(corev1.SeccompProfileTypeRuntimeDefault))
		})
	})
})
