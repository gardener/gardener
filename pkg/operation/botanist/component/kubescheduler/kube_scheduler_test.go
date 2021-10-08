// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubescheduler_test

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Masterminds/semver"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/kubescheduler"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

var _ = Describe("KubeScheduler", func() {
	var (
		ctrl          *gomock.Controller
		c             *mockclient.MockClient
		kubeScheduler Interface

		ctx                    = context.TODO()
		fakeErr                = fmt.Errorf("fake error")
		namespace              = "shoot--foo--bar"
		version                = "1.17.2"
		semverVersion, _       = semver.NewVersion(version)
		image                  = "k8s.gcr.io/kube-scheduler:v1.17.2"
		replicas         int32 = 1

		configEmpty *gardencorev1beta1.KubeSchedulerConfig
		configFull  = &gardencorev1beta1.KubeSchedulerConfig{KubernetesConfig: gardencorev1beta1.KubernetesConfig{FeatureGates: map[string]bool{"Foo": true, "Bar": false, "Baz": false}}, KubeMaxPDVols: pointer.String("23")}

		secretNameKubeconfig     = "kubeconfig-secret"
		secretChecksumKubeconfig = "1234"
		secretNameServer         = "server-secret"
		secretChecksumServer     = "5678"
		secrets                  = Secrets{
			Kubeconfig: component.Secret{Name: secretNameKubeconfig, Checksum: secretChecksumKubeconfig},
			Server:     component.Secret{Name: secretNameServer, Checksum: secretChecksumServer},
		}

		vpaName                   = "kube-scheduler-vpa"
		serviceName               = "kube-scheduler"
		deploymentName            = "kube-scheduler"
		managedResourceName       = "shoot-core-kube-scheduler"
		managedResourceSecretName = "managedresource-shoot-core-kube-scheduler"

		configMapFor = func(version string) *corev1.ConfigMap {
			componentConfigYAML := componentConfigYAMLForKubernetesVersion(version)
			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-scheduler-config", Namespace: namespace},
				Data:       map[string]string{"config.yaml": componentConfigYAML},
			}
			Expect(kutil.MakeUnique(cm)).To(Succeed())
			return cm
		}
		vpaUpdateMode = autoscalingv1beta2.UpdateModeAuto
		vpa           = &autoscalingv1beta2.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace},
			Spec: autoscalingv1beta2.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       deploymentName,
				},
				UpdatePolicy: &autoscalingv1beta2.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &autoscalingv1beta2.PodResourcePolicy{
					ContainerPolicies: []autoscalingv1beta2.ContainerResourcePolicy{
						{
							ContainerName: autoscalingv1beta2.DefaultContainerResourcePolicy,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("20m"),
								corev1.ResourceMemory: resource.MustParse("50Mi"),
							},
						},
					},
				},
			},
		}
		serviceFor = func(version string) *corev1.Service {
			return &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceName,
					Namespace: namespace,
					Labels: map[string]string{
						"app":  "kubernetes",
						"role": "scheduler",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app":  "kubernetes",
						"role": "scheduler",
					},
					Type: corev1.ServiceTypeClusterIP,
					Ports: []corev1.ServicePort{
						{
							Name:     "metrics",
							Protocol: corev1.ProtocolTCP,
							Port:     10259,
						},
					},
				},
			}
		}
		deploymentFor = func(version string, config *gardencorev1beta1.KubeSchedulerConfig) *appsv1.Deployment {
			var env []corev1.EnvVar
			if config != nil && config.KubeMaxPDVols != nil {
				env = append(env, corev1.EnvVar{
					Name:  "KUBE_MAX_PD_VOLS",
					Value: *config.KubeMaxPDVols,
				})
			}

			configMap := configMapFor(version)

			deploy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: namespace,
					Labels: map[string]string{
						"app":                 "kubernetes",
						"role":                "scheduler",
						"gardener.cloud/role": "controlplane",
					},
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: pointer.Int32(1),
					Replicas:             &replicas,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":  "kubernetes",
							"role": "scheduler",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"checksum/secret-" + secretNameKubeconfig: secretChecksumKubeconfig,
								"checksum/secret-" + secretNameServer:     secretChecksumServer,
							},
							Labels: map[string]string{
								"app":                                "kubernetes",
								"role":                               "scheduler",
								"gardener.cloud/role":                "controlplane",
								"maintenance.gardener.cloud/restart": "true",
								"networking.gardener.cloud/to-dns":   "allowed",
								"networking.gardener.cloud/to-shoot-apiserver": "allowed",
								"networking.gardener.cloud/from-prometheus":    "allowed",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:            "kube-scheduler",
									Image:           image,
									ImagePullPolicy: corev1.PullIfNotPresent,
									Command:         commandForKubernetesVersion(version, 10259, featureGateFlags(config)...),
									LivenessProbe: &corev1.Probe{
										Handler: corev1.Handler{
											HTTPGet: &corev1.HTTPGetAction{
												Path:   "/healthz",
												Scheme: corev1.URISchemeHTTPS,
												Port:   intstr.FromInt(10259),
											},
										},
										SuccessThreshold:    1,
										FailureThreshold:    2,
										InitialDelaySeconds: 15,
										PeriodSeconds:       10,
										TimeoutSeconds:      15,
									},
									Ports: []corev1.ContainerPort{
										{
											Name:          "metrics",
											ContainerPort: 10259,
											Protocol:      corev1.ProtocolTCP,
										},
									},
									Env: env,
									Resources: corev1.ResourceRequirements{
										Requests: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("23m"),
											corev1.ResourceMemory: resource.MustParse("64Mi"),
										},
										Limits: corev1.ResourceList{
											corev1.ResourceCPU:    resource.MustParse("400m"),
											corev1.ResourceMemory: resource.MustParse("512Mi"),
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      secretNameKubeconfig,
											MountPath: "/var/lib/kube-scheduler",
										},
										{
											Name:      secretNameServer,
											MountPath: "/var/lib/kube-scheduler-server",
										},
										{
											Name:      "kube-scheduler-config",
											MountPath: "/var/lib/kube-scheduler-config",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: secretNameKubeconfig,
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: secretNameKubeconfig,
										},
									},
								},
								{
									Name: secretNameServer,
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: secretNameServer,
										},
									},
								},
								{
									Name: "kube-scheduler-config",
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: configMap.Name,
											},
										},
									},
								},
							},
						},
					},
				},
			}

			Expect(references.InjectAnnotations(deploy)).To(Succeed())
			return deploy
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		kubeScheduler = New(c, namespace, semverVersion, image, replicas, configEmpty)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		Context("missing secret information", func() {
			It("should return an error because the kubeconfig secret information is not provided", func() {
				Expect(kubeScheduler.Deploy(ctx)).To(MatchError(ContainSubstring("missing kubeconfig secret information")))
			})

			It("should return an error because the kubeconfig secret information is not provided", func() {
				kubeScheduler.SetSecrets(Secrets{Kubeconfig: component.Secret{Name: secretNameKubeconfig, Checksum: secretChecksumKubeconfig}})
				Expect(kubeScheduler.Deploy(ctx)).To(MatchError(ContainSubstring("missing server secret information")))
			})
		})

		Context("secret information available", func() {
			BeforeEach(func() {
				kubeScheduler.SetSecrets(secrets)
			})

			It("should fail because the configmap cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, configMapFor(version)).Return(fakeErr),
				)

				Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the service cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, configMapFor(version)),
					c.EXPECT().Get(ctx, kutil.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).Return(fakeErr),
				)

				Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the deployment cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, configMapFor(version)),
					c.EXPECT().Get(ctx, kutil.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).Return(fakeErr),
				)

				Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the vpa cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, configMapFor(version)),
					c.EXPECT().Get(ctx, kutil.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{}), gomock.Any()).Return(fakeErr),
				)

				Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the managed resource cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, configMapFor(version)),
					c.EXPECT().Get(ctx, kutil.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{}), gomock.Any()),
					c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: namespace}}).Return(fakeErr),
				)

				Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the managed resource secret cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, configMapFor(version)),
					c.EXPECT().Get(ctx, kutil.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{}), gomock.Any()),
					c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: namespace}}),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: managedResourceSecretName, Namespace: namespace}}).Return(fakeErr),
				)

				Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the legacy config map cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Create(ctx, configMapFor(version)),
					c.EXPECT().Get(ctx, kutil.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{}), gomock.Any()),
					c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: namespace}}),
					c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: managedResourceSecretName, Namespace: namespace}}),
					c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "kube-scheduler-config"}}).Return(fakeErr),
				)

				Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			DescribeTable("success tests for various kubernetes versions",
				func(version string, config *gardencorev1beta1.KubeSchedulerConfig) {
					semverVersion, err := semver.NewVersion(version)
					Expect(err).NotTo(HaveOccurred())

					kubeScheduler = New(c, namespace, semverVersion, image, replicas, config)
					kubeScheduler.SetSecrets(secrets)

					gomock.InOrder(
						c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{}), gomock.Any()).
							Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
								Expect(obj).To(DeepEqual(configMapFor(version)))
							}),
						c.EXPECT().Get(ctx, kutil.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
							Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(serviceFor(version)))
							}),
						c.EXPECT().Get(ctx, kutil.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
							Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(deploymentFor(version, config)))
							}),
						c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
						c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{}), gomock.Any()).
							Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
								Expect(obj).To(DeepEqual(vpa))
							}),
					)

					gomock.InOrder(
						c.EXPECT().Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: namespace}}),
						c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: managedResourceSecretName, Namespace: namespace}}),
						c.EXPECT().Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "kube-scheduler-config"}}),
					)

					Expect(kubeScheduler.Deploy(ctx)).To(Succeed())
				},

				Entry("kubernetes 1.15 w/o config", "1.15.5", configEmpty),
				Entry("kubernetes 1.15 w/ full config", "1.15.5", configFull),
				Entry("kubernetes 1.16 w/o config", "1.16.6", configEmpty),
				Entry("kubernetes 1.16 w/ full config", "1.16.6", configFull),
				Entry("kubernetes 1.17 w/o config", "1.17.7", configEmpty),
				Entry("kubernetes 1.17 w/ full config", "1.17.7", configFull),
				Entry("kubernetes 1.18 w/o config", "1.18.8", configEmpty),
				Entry("kubernetes 1.18 w/ full config", "1.18.8", configFull),
				Entry("kubernetes 1.19 w/o config", "1.19.9", configEmpty),
				Entry("kubernetes 1.19 w/ full config", "1.19.9", configFull),
				Entry("kubernetes 1.20 w/o config", "1.20.9", configEmpty),
				Entry("kubernetes 1.20 w/ full config", "1.20.9", configFull),
				Entry("kubernetes 1.21 w/o config", "1.21.3", configEmpty),
				Entry("kubernetes 1.21 w/ full config", "1.21.3", configFull),
				Entry("kubernetes 1.22 w/o config", "1.22.1", configEmpty),
				Entry("kubernetes 1.22 w/ full config", "1.22.1", configFull),
			)
		})
	})

	Describe("#Destroy", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(kubeScheduler.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(kubeScheduler.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should return nil as it's not implemented as of now", func() {
			Expect(kubeScheduler.WaitCleanup(ctx)).To(Succeed())
		})
	})
})

func componentConfigYAMLForKubernetesVersion(version string) string {
	var apiVersion string
	if k8sVersionGreaterEqual122, _ := versionutils.CompareVersions(version, ">=", "1.22"); k8sVersionGreaterEqual122 {
		apiVersion = "kubescheduler.config.k8s.io/v1beta2"
	} else if k8sVersionGreaterEqual119, _ := versionutils.CompareVersions(version, ">=", "1.19"); k8sVersionGreaterEqual119 {
		apiVersion = "kubescheduler.config.k8s.io/v1beta1"
	} else if k8sVersionGreaterEqual118, _ := versionutils.CompareVersions(version, ">=", "1.18"); k8sVersionGreaterEqual118 {
		apiVersion = "kubescheduler.config.k8s.io/v1alpha2"
	} else {
		apiVersion = "kubescheduler.config.k8s.io/v1alpha1"
	}

	return `apiVersion: ` + apiVersion + `
kind: KubeSchedulerConfiguration
clientConnection:
  kubeconfig: /var/lib/kube-scheduler/kubeconfig
leaderElection:
  leaderElect: true`
}

func commandForKubernetesVersion(version string, port int32, featureGateFlags ...string) []string {
	var command []string

	if k8sVersionLessThan117, _ := versionutils.CompareVersions(version, "<", "1.17"); k8sVersionLessThan117 {
		command = append(command, "/hyperkube", "kube-scheduler")
	} else {
		command = append(command, "/usr/local/bin/kube-scheduler")
	}

	command = append(command, "--config=/var/lib/kube-scheduler-config/config.yaml")

	command = append(
		command,
		"--authentication-kubeconfig=/var/lib/kube-scheduler/kubeconfig",
		"--authorization-kubeconfig=/var/lib/kube-scheduler/kubeconfig",
		"--client-ca-file=/var/lib/kube-scheduler-server/ca.crt",
		"--tls-cert-file=/var/lib/kube-scheduler-server/kube-scheduler-server.crt",
		"--tls-private-key-file=/var/lib/kube-scheduler-server/kube-scheduler-server.key",
		"--secure-port="+strconv.Itoa(int(port)),
		"--port=0",
	)

	command = append(command, featureGateFlags...)
	command = append(command, "--v=2")

	return command
}

func featureGateFlags(config *gardencorev1beta1.KubeSchedulerConfig) []string {
	var out []string

	if config != nil && config.FeatureGates != nil {
		out = append(out, kutil.FeatureGatesToCommandLineParameter(config.FeatureGates))
	}

	return out
}
