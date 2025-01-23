// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	fakerestclient "k8s.io/client-go/rest/fake"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mockcorev1 "github.com/gardener/gardener/third_party/mock/client-go/core/v1"
	mockio "github.com/gardener/gardener/third_party/mock/go/io"
)

var _ = Describe("Pod Utils", func() {
	var (
		ctx        = context.TODO()
		fakeClient client.Client

		podSpec corev1.PodSpec
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()

		podSpec = corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name: "init1",
				},
				{
					Name: "init2",
				},
			},
			Containers: []corev1.Container{
				{
					Name: "container1",
				},
				{
					Name: "container2",
				},
			},
		}
	})

	Describe("#VisitPodSpec", func() {
		It("should do nothing because object type is not handled", func() {
			Expect(VisitPodSpec(&corev1.Service{}, nil)).To(MatchError(ContainSubstring("unhandled object type")))
		})

		test := func(obj runtime.Object, podSpec *corev1.PodSpec) {
			It("should visit and mutate PodSpec", Offset(1), func() {
				Expect(VisitPodSpec(obj, func(podSpec *corev1.PodSpec) {
					podSpec.RestartPolicy = corev1.RestartPolicyOnFailure
				})).To(Succeed())

				Expect(podSpec.RestartPolicy).To(Equal(corev1.RestartPolicyOnFailure))
			})
		}

		Context("corev1.Pod", func() {
			var obj = &corev1.Pod{
				Spec: podSpec,
			}
			test(obj, &obj.Spec)
		})

		Context("appsv1.Deployment", func() {
			var obj = &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("appsv1beta2.Deployment", func() {
			var obj = &appsv1beta2.Deployment{
				Spec: appsv1beta2.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("appsv1beta1.Deployment", func() {
			var obj = &appsv1beta1.Deployment{
				Spec: appsv1beta1.DeploymentSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("appsv1.StatefulSet", func() {
			var obj = &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("appsv1beta2.StatefulSet", func() {
			var obj = &appsv1beta2.StatefulSet{
				Spec: appsv1beta2.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("appsv1beta1.StatefulSet", func() {
			var obj = &appsv1beta1.StatefulSet{
				Spec: appsv1beta1.StatefulSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("appsv1.DaemonSet", func() {
			var obj = &appsv1.DaemonSet{
				Spec: appsv1.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("appsv1beta2.DaemonSet", func() {
			var obj = &appsv1beta2.DaemonSet{
				Spec: appsv1beta2.DaemonSetSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("batchv1.Job", func() {
			var obj = &batchv1.Job{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: podSpec,
					},
				},
			}
			test(obj, &obj.Spec.Template.Spec)
		})

		Context("batchv1.CronJob", func() {
			var obj = &batchv1.CronJob{
				Spec: batchv1.CronJobSpec{
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: podSpec,
							},
						},
					},
				},
			}
			test(obj, &obj.Spec.JobTemplate.Spec.Template.Spec)
		})

		Context("batchv1beta1.CronJob", func() {
			var obj = &batchv1beta1.CronJob{
				Spec: batchv1beta1.CronJobSpec{
					JobTemplate: batchv1beta1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: corev1.PodTemplateSpec{
								Spec: podSpec,
							},
						},
					},
				},
			}
			test(obj, &obj.Spec.JobTemplate.Spec.Template.Spec)
		})
	})

	Describe("#VisitContainers", func() {
		It("should do nothing if there are no containers", func() {
			podSpec.InitContainers = nil
			podSpec.Containers = nil
			VisitContainers(&podSpec, func(_ *corev1.Container) {
				Fail("called visitor")
			})
		})

		It("should visit and mutate all containers if no names are given", func() {
			VisitContainers(&podSpec, func(container *corev1.Container) {
				container.TerminationMessagePath = "visited"
			})

			for _, container := range append(podSpec.InitContainers, podSpec.Containers...) {
				Expect(container.TerminationMessagePath).To(Equal("visited"), "should have visited and mutated container %s", container.Name)
			}
		})

		It("should visit and mutate only containers with matching names", func() {
			names := sets.New(podSpec.InitContainers[0].Name, podSpec.Containers[0].Name)

			VisitContainers(&podSpec, func(container *corev1.Container) {
				container.TerminationMessagePath = "visited"
			}, names.UnsortedList()...)

			for _, container := range append(podSpec.InitContainers, podSpec.Containers...) {
				if names.Has(container.Name) {
					Expect(container.TerminationMessagePath).To(Equal("visited"), "should have visited and mutated container %s", container.Name)
				} else {
					Expect(container.TerminationMessagePath).To(BeEmpty(), "should not have visited and mutated container %s", container.Name)
				}
			}
		})
	})

	Describe("#AddVolume", func() {
		var volume corev1.Volume

		BeforeEach(func() {
			volume = corev1.Volume{
				Name: "volume",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "secret",
					},
				},
			}
		})

		It("should add the volume if there are none", func() {
			podSpec.Volumes = nil

			AddVolume(&podSpec, *volume.DeepCopy(), false)

			Expect(podSpec.Volumes).To(ConsistOf(volume))
		})

		It("should add the volume if it is not present yet", func() {
			otherVolume := *volume.DeepCopy()
			otherVolume.Name += "-other"
			podSpec.Volumes = []corev1.Volume{otherVolume}

			AddVolume(&podSpec, *volume.DeepCopy(), false)

			Expect(podSpec.Volumes).To(ConsistOf(otherVolume, volume))
		})

		It("should do nothing if the volume is already present (overwrite=false)", func() {
			otherVolume := *volume.DeepCopy()
			otherVolume.Secret.SecretName += "-other"
			podSpec.Volumes = []corev1.Volume{otherVolume}

			AddVolume(&podSpec, *volume.DeepCopy(), false)

			Expect(podSpec.Volumes).To(ConsistOf(otherVolume))
		})

		It("should overwrite the volume if it is already present (overwrite=false)", func() {
			otherVolume := *volume.DeepCopy()
			otherVolume.Secret.SecretName += "-other"
			podSpec.Volumes = []corev1.Volume{otherVolume}

			AddVolume(&podSpec, *volume.DeepCopy(), true)

			Expect(podSpec.Volumes).To(ConsistOf(volume))
		})
	})

	Describe("#AddVolumeMount", func() {
		var (
			container   corev1.Container
			volumeMount corev1.VolumeMount
		)

		BeforeEach(func() {
			container = podSpec.Containers[0]
			volumeMount = corev1.VolumeMount{
				Name:      "volume",
				MountPath: "path",
			}
		})

		It("should add the volumeMount if there are none", func() {
			container.VolumeMounts = nil

			AddVolumeMount(&container, *volumeMount.DeepCopy(), false)

			Expect(container.VolumeMounts).To(ConsistOf(volumeMount))
		})

		It("should add the volumeMount if it is not present yet", func() {
			otherVolumeMount := *volumeMount.DeepCopy()
			otherVolumeMount.Name += "-other"
			container.VolumeMounts = []corev1.VolumeMount{otherVolumeMount}

			AddVolumeMount(&container, *volumeMount.DeepCopy(), false)

			Expect(container.VolumeMounts).To(ConsistOf(otherVolumeMount, volumeMount))
		})

		It("should do nothing if the volumeMount is already present (overwrite=false)", func() {
			otherVolumeMount := *volumeMount.DeepCopy()
			otherVolumeMount.MountPath += "-other"
			container.VolumeMounts = []corev1.VolumeMount{otherVolumeMount}

			AddVolumeMount(&container, *volumeMount.DeepCopy(), false)

			Expect(container.VolumeMounts).To(ConsistOf(otherVolumeMount))
		})

		It("should overwrite the volumeMount if it is already present (overwrite=false)", func() {
			otherVolumeMount := *volumeMount.DeepCopy()
			otherVolumeMount.MountPath += "-other"
			container.VolumeMounts = []corev1.VolumeMount{otherVolumeMount}

			AddVolumeMount(&container, *volumeMount.DeepCopy(), true)

			Expect(container.VolumeMounts).To(ConsistOf(volumeMount))
		})
	})

	Describe("#AddEnvVar", func() {
		var (
			container corev1.Container
			envVar    corev1.EnvVar
		)

		BeforeEach(func() {
			container = podSpec.Containers[0]
			envVar = corev1.EnvVar{
				Name:  "env",
				Value: "var",
			}
		})

		It("should add the envVar if there are none", func() {
			container.Env = nil

			AddEnvVar(&container, *envVar.DeepCopy(), false)

			Expect(container.Env).To(ConsistOf(envVar))
		})

		It("should add the envVar if it is not present yet", func() {
			otherEnvVar := *envVar.DeepCopy()
			otherEnvVar.Name += "-other"
			container.Env = []corev1.EnvVar{otherEnvVar}

			AddEnvVar(&container, *envVar.DeepCopy(), false)

			Expect(container.Env).To(ConsistOf(otherEnvVar, envVar))
		})

		It("should do nothing if the envVar is already present (overwrite=false)", func() {
			otherEnvVar := *envVar.DeepCopy()
			otherEnvVar.Value += "-other"
			container.Env = []corev1.EnvVar{otherEnvVar}

			AddEnvVar(&container, *envVar.DeepCopy(), false)

			Expect(container.Env).To(ConsistOf(otherEnvVar))
		})

		It("should overwrite the envVar if it is already present (overwrite=false)", func() {
			otherEnvVar := *envVar.DeepCopy()
			otherEnvVar.Value += "-other"
			container.Env = []corev1.EnvVar{otherEnvVar}

			AddEnvVar(&container, *envVar.DeepCopy(), true)

			Expect(container.Env).To(ConsistOf(envVar))
		})
	})

	Describe("#HasEnvVar", func() {
		var (
			container  corev1.Container
			envVarName string
		)

		BeforeEach(func() {
			container = podSpec.Containers[0]
			envVarName = "env"
		})

		It("should return false if there are no env vars", func() {
			container.Env = nil

			Expect(HasEnvVar(container, envVarName)).To(BeFalse())
		})

		It("should return false if there are only other env vars", func() {
			container.Env = []corev1.EnvVar{
				{Name: "env1"},
				{Name: "env2"},
			}

			Expect(HasEnvVar(container, envVarName)).To(BeFalse())
		})

		It("should return true if it has the env var", func() {
			container.Env = []corev1.EnvVar{
				{Name: "env1"},
				{Name: "env"},
			}

			Expect(HasEnvVar(container, envVarName)).To(BeTrue())
		})
	})

	Describe("#GetDeploymentForPod", func() {
		var (
			deployment *appsv1.Deployment
			replicaSet *appsv1.ReplicaSet
		)

		BeforeEach(func() {
			deployment = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "deployment", Namespace: namespace}}
			replicaSet = &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "replicaset", Namespace: namespace, OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: deployment.Name}}}}
		})

		It("should return nil because pod has no owner reference to a ReplicaSet", func() {
			deployment, err := GetDeploymentForPod(ctx, fakeClient, namespace, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(deployment).To(BeNil())
		})

		It("should return an error because ReplicaSet does not exist", func() {
			deployment, err := GetDeploymentForPod(ctx, fakeClient, namespace, []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: replicaSet.Name}})
			Expect(err).To(MatchError(ContainSubstring("failed reading ReplicaSet")))
			Expect(deployment).To(BeNil())
		})

		It("should return nil because ReplicaSet has no owner reference to a Deployment", func() {
			replicaSet.OwnerReferences = nil
			Expect(fakeClient.Create(ctx, replicaSet)).To(Succeed())

			deployment, err := GetDeploymentForPod(ctx, fakeClient, namespace, []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: replicaSet.Name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(deployment).To(BeNil())
		})

		It("should return an error because Deployment does not exist", func() {
			Expect(fakeClient.Create(ctx, replicaSet)).To(Succeed())

			deployment, err := GetDeploymentForPod(ctx, fakeClient, namespace, []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: replicaSet.Name}})
			Expect(err).To(MatchError(ContainSubstring("failed reading Deployment")))
			Expect(deployment).To(BeNil())
		})

		It("should return the owning deployment", func() {
			Expect(fakeClient.Create(ctx, replicaSet)).To(Succeed())
			Expect(fakeClient.Create(ctx, deployment)).To(Succeed())

			deployment, err := GetDeploymentForPod(ctx, fakeClient, namespace, []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "ReplicaSet", Name: replicaSet.Name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(deployment).To(Equal(deployment))
		})
	})

	Describe("#DeleteStalePods", func() {
		It("should delete the stale pods", func() {
			var (
				normalPod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "normal", Namespace: "default"},
				}
				stalePod = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "stale", Namespace: "default"},
					Status:     corev1.PodStatus{Reason: "Evicted"},
				}
				pods = []corev1.Pod{*normalPod, *stalePod}
				// There is no good way with the fake client to test the deletion of the pods stuck in termination
				// We'd have to use a mock client, but we actually want to avoid its usage.
			)

			Expect(fakeClient.Create(ctx, normalPod)).To(Succeed())
			Expect(fakeClient.Create(ctx, stalePod)).To(Succeed())

			Expect(DeleteStalePods(ctx, logr.Discard(), fakeClient, pods)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(normalPod), &corev1.Pod{})).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(stalePod), &corev1.Pod{})).To(BeNotFoundError())
		})
	})
})

var _ = Describe("Pods", func() {
	var (
		ctx  context.Context
		ctrl *gomock.Controller
		pods *mockcorev1.MockPodInterface
	)

	BeforeEach(func() {
		ctx = context.Background()
		ctrl = gomock.NewController(GinkgoT())
		pods = mockcorev1.NewMockPodInterface(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#GetPodLogs", func() {
		It("should read all pod logs and close the stream", func() {
			const name = "name"
			var (
				options = &corev1.PodLogOptions{}
				logs    = []byte("logs")
				body    = mockio.NewMockReadCloser(ctrl)
				client  = fakerestclient.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
				})
			)

			gomock.InOrder(
				pods.EXPECT().GetLogs(name, options).Return(rest.NewRequestWithClient(&url.URL{}, "", rest.ClientContentConfig{}, client)),
				body.EXPECT().Read(gomock.Any()).DoAndReturn(func(data []byte) (int, error) {
					copy(data, logs)
					return len(logs), io.EOF
				}),
				body.EXPECT().Close(),
			)

			actual, err := GetPodLogs(ctx, pods, name, options.DeepCopy())
			Expect(err).NotTo(HaveOccurred())
			Expect(actual).To(Equal(logs))
		})
	})
})
