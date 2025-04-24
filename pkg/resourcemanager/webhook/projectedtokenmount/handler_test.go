// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package projectedtokenmount_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/resourcemanager/webhook/projectedtokenmount"
)

var _ = Describe("Handler", func() {
	var (
		ctx        = context.TODO()
		log        logr.Logger
		fakeClient client.Client
		handler    *Handler

		pod            *corev1.Pod
		serviceAccount *corev1.ServiceAccount

		namespace          = "some-namespace"
		serviceAccountName = "some-service-account"
		expirationSeconds  int64
		mode               int32
	)

	BeforeEach(func() {
		expirationSeconds = 1337
		mode = 420

		ctx = admission.NewContextWithRequest(ctx, admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Namespace: namespace}})
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		handler = &Handler{Logger: log, TargetReader: fakeClient, ExpirationSeconds: expirationSeconds}

		pod = &corev1.Pod{
			Spec: corev1.PodSpec{
				ServiceAccountName: serviceAccountName,
				Containers:         []corev1.Container{{}, {}},
				InitContainers:     []corev1.Container{{}},
			},
		}
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceAccountName,
				Namespace: namespace,
			},
			AutomountServiceAccountToken: ptr.To(false),
		}
	})

	Describe("#Default", func() {
		DescribeTable("should not mutate because preconditions are not met",
			func(mutatePod func()) {
				mutatePod()

				Expect(handler.Default(ctx, pod)).To(Succeed())
				Expect(pod.Spec.Volumes).To(BeEmpty())
				Expect(pod.Spec.Containers[0].VolumeMounts).To(BeEmpty())
				Expect(pod.Spec.Containers[1].VolumeMounts).To(BeEmpty())
				Expect(pod.Spec.InitContainers[0].VolumeMounts).To(BeEmpty())
				Expect(pod.Spec.SecurityContext).To(BeNil())
			},

			Entry("service account name is empty",
				func() { pod.Spec.ServiceAccountName = "" },
			),

			Entry("service account name is default",
				func() { pod.Spec.ServiceAccountName = "default" },
			),
		)

		It("should return an error because the service account cannot be read", func() {
			Expect(handler.Default(ctx, pod)).To(MatchError(ContainSubstring(`serviceaccounts "` + serviceAccountName + `" not found`)))
		})

		DescribeTable("should not mutate because service account's preconditions are not met",
			func(mutate func()) {
				mutate()
				Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())

				Expect(handler.Default(ctx, pod)).To(Succeed())
				Expect(pod.Spec.Volumes).To(BeEmpty())
				Expect(pod.Spec.Containers[0].VolumeMounts).To(BeEmpty())
				Expect(pod.Spec.Containers[1].VolumeMounts).To(BeEmpty())
				Expect(pod.Spec.InitContainers[0].VolumeMounts).To(BeEmpty())
				Expect(pod.Spec.SecurityContext).To(BeNil())
			},

			Entry("ServiceAccount's automountServiceAccountToken=nil", func() {
				serviceAccount.AutomountServiceAccountToken = nil
			}),
			Entry("ServiceAccount's automountServiceAccountToken=true", func() {
				serviceAccount.AutomountServiceAccountToken = ptr.To(true)
			}),
		)

		Context("when service account exists", func() {
			BeforeEach(func() {
				Expect(fakeClient.Create(ctx, serviceAccount)).To(Succeed())
			})

			It("should not mutate because pod explicitly disables the service account mount", func() {
				pod.Spec.AutomountServiceAccountToken = ptr.To(false)

				Expect(handler.Default(ctx, pod)).To(Succeed())
				Expect(pod.Spec.Volumes).To(BeEmpty())
				Expect(pod.Spec.Containers[0].VolumeMounts).To(BeEmpty())
				Expect(pod.Spec.Containers[1].VolumeMounts).To(BeEmpty())
				Expect(pod.Spec.InitContainers[0].VolumeMounts).To(BeEmpty())
				Expect(pod.Spec.SecurityContext).To(BeNil())
			})

			It("should not mutate because pod already has a projected token volume", func() {
				pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{Name: "kube-api-access-2138h"})

				Expect(handler.Default(ctx, pod)).To(Succeed())
				Expect(pod.Spec.Volumes).To(ConsistOf(corev1.Volume{Name: "kube-api-access-2138h"}))
				Expect(pod.Spec.Containers[0].VolumeMounts).To(BeEmpty())
				Expect(pod.Spec.Containers[1].VolumeMounts).To(BeEmpty())
				Expect(pod.Spec.InitContainers[0].VolumeMounts).To(BeEmpty())
				Expect(pod.Spec.SecurityContext).To(BeNil())
			})

			Context("should mutate", func() {
				BeforeEach(func() {
					expirationSeconds = 1337
					mode = 420
					pod.Annotations = map[string]string{}
				})

				AfterEach(func() {
					Expect(handler.Default(ctx, pod)).To(Succeed())
					Expect(pod.Spec.Volumes).To(ConsistOf(corev1.Volume{
						Name: "kube-api-access-gardener",
						VolumeSource: corev1.VolumeSource{
							Projected: &corev1.ProjectedVolumeSource{
								DefaultMode: ptr.To[int32](mode),
								Sources: []corev1.VolumeProjection{
									{
										ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
											ExpirationSeconds: &expirationSeconds,
											Path:              "token",
										},
									},
									{
										ConfigMap: &corev1.ConfigMapProjection{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "kube-root-ca.crt",
											},
											Items: []corev1.KeyToPath{{
												Key:  "ca.crt",
												Path: "ca.crt",
											}},
										},
									},
									{
										DownwardAPI: &corev1.DownwardAPIProjection{
											Items: []corev1.DownwardAPIVolumeFile{{
												FieldRef: &corev1.ObjectFieldSelector{
													APIVersion: "v1",
													FieldPath:  "metadata.namespace",
												},
												Path: "namespace",
											}},
										},
									},
								},
							},
						},
					}))
					Expect(pod.Spec.Containers[0].VolumeMounts).To(ConsistOf(corev1.VolumeMount{
						Name:      "kube-api-access-gardener",
						ReadOnly:  true,
						MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
					}))
					Expect(pod.Spec.Containers[1].VolumeMounts).To(ConsistOf(corev1.VolumeMount{
						Name:      "kube-api-access-gardener",
						ReadOnly:  true,
						MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
					}))
					Expect(pod.Spec.InitContainers[0].VolumeMounts).To(ConsistOf(corev1.VolumeMount{
						Name:      "kube-api-access-gardener",
						ReadOnly:  true,
						MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
					}))
				})

				It("normal case", func() {})

				It("with overridden expiration seconds", func() {
					expirationSeconds = 8998
					pod.Annotations = map[string]string{"projected-token-mount.resources.gardener.cloud/expiration-seconds": "8998"}
				})

				It("with overridden file mode", func() {
					mode = 416
					pod.Annotations = map[string]string{"projected-token-mount.resources.gardener.cloud/file-mode": "416"}
				})
			})
		})
	})
})
