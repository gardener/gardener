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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/operation/botanist/controlplane/kubescheduler"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	"github.com/gardener/gardener/test/gomega"

	"github.com/Masterminds/semver"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("KubeScheduler", func() {
	var (
		ctrl          *gomock.Controller
		c             *mockclient.MockClient
		kubeScheduler KubeScheduler

		ctx                    = context.TODO()
		fakeErr                = fmt.Errorf("fake error")
		namespace              = "shoot--foo--bar"
		version                = "1.17.2"
		semverVersion, _       = semver.NewVersion(version)
		image                  = "k8s.gcr.io/kube-scheduler:v1.17.2"
		replicas         int32 = 1

		configEmpty *gardencorev1beta1.KubeSchedulerConfig
		configFull  = &gardencorev1beta1.KubeSchedulerConfig{KubernetesConfig: gardencorev1beta1.KubernetesConfig{FeatureGates: map[string]bool{"Foo": true, "Bar": false, "Baz": false}}, KubeMaxPDVols: pointer.StringPtr("23")}

		secretNameKubeconfig     = "kubeconfig-secret"
		secretChecksumKubeconfig = "1234"
		secretNameServer         = "server-secret"
		secretChecksumServer     = "5678"
		vpaUpdateMode            = autoscalingv1beta2.UpdateModeAuto

		configMapName  = "kube-scheduler-config"
		vpaName        = "kube-scheduler-vpa"
		serviceName    = "kube-scheduler"
		deploymentName = "kube-scheduler"

		configMapFor = func(version string) *corev1.ConfigMap {
			componentConfigYAML, _ := componentConfigYAMLForKubernetesVersion(version)
			return &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: configMapName, Namespace: namespace},
				Data:       map[string]string{"config.yaml": componentConfigYAML},
			}
		}
		vpa = &autoscalingv1beta2.VerticalPodAutoscaler{
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
							Port:     portForKubernetesVersion(version),
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

			_, componentConfigChecksum := componentConfigYAMLForKubernetesVersion(version)

			return &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: namespace,
					Labels: map[string]string{
						"app":                     "kubernetes",
						"role":                    "scheduler",
						"garden.sapcloud.io/role": "controlplane",
					},
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: pointer.Int32Ptr(0),
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
								"checksum/configmap-componentconfig":      componentConfigChecksum,
								"checksum/secret-" + secretNameKubeconfig: secretChecksumKubeconfig,
								"checksum/secret-" + secretNameServer:     secretChecksumServer,
							},
							Labels: map[string]string{
								"app":                                "kubernetes",
								"role":                               "scheduler",
								"garden.sapcloud.io/role":            "controlplane",
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
									Command:         commandForKubernetesVersion(version, portForKubernetesVersion(version), featureGateFlagsForKubernetesVersion(version, config)...),
									LivenessProbe: &corev1.Probe{
										Handler: corev1.Handler{
											HTTPGet: &corev1.HTTPGetAction{
												Path:   "/healthz",
												Scheme: probeSchemeForKubernetesVersion(version),
												Port:   intstr.FromInt(int(portForKubernetesVersion(version))),
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
											ContainerPort: portForKubernetesVersion(version),
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
											Name:      configMapName,
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
									Name: configMapName,
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: configMapName,
											},
										},
									},
								},
							},
						},
					},
				},
			}
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
				kubeScheduler.SetSecrets(Secrets{Kubeconfig: Secret{Name: secretNameKubeconfig, Checksum: secretChecksumKubeconfig}})
				Expect(kubeScheduler.Deploy(ctx)).To(MatchError(ContainSubstring("missing server secret information")))
			})
		})

		Context("secret information available", func() {
			BeforeEach(func() {
				kubeScheduler.SetSecrets(Secrets{
					Kubeconfig: Secret{Name: secretNameKubeconfig, Checksum: secretChecksumKubeconfig},
					Server:     Secret{Name: secretNameServer, Checksum: secretChecksumServer},
				})
			})

			It("should fail because the configmap cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, configMapName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).Return(fakeErr),
				)

				Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the vpa cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, configMapName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})).Return(fakeErr),
				)

				Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the service cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, configMapName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})).Return(fakeErr),
				)

				Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail because the deployment cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, configMapName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(fakeErr),
				)

				Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
			})

			DescribeTable("success tests for various kubernetes versions",
				func(version string, config *gardencorev1beta1.KubeSchedulerConfig) {
					semverVersion, err := semver.NewVersion(version)
					Expect(err).NotTo(HaveOccurred())

					kubeScheduler = New(c, namespace, semverVersion, image, replicas, config)
					kubeScheduler.SetSecrets(Secrets{
						Kubeconfig: Secret{Name: secretNameKubeconfig, Checksum: secretChecksumKubeconfig},
						Server:     Secret{Name: secretNameServer, Checksum: secretChecksumServer},
					})

					gomock.InOrder(
						c.EXPECT().Get(ctx, kutil.Key(namespace, configMapName), gomock.AssignableToTypeOf(&corev1.ConfigMap{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
							Expect(obj).To(gomega.DeepDerivativeEqual(configMapFor(version)))
						}),
						c.EXPECT().Get(ctx, kutil.Key(namespace, vpaName), gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&autoscalingv1beta2.VerticalPodAutoscaler{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
							Expect(obj).To(gomega.DeepDerivativeEqual(vpa))
						}),
						c.EXPECT().Get(ctx, kutil.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Service{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
							Expect(obj).To(gomega.DeepDerivativeEqual(serviceFor(version)))
						}),
						c.EXPECT().Get(ctx, kutil.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
						c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Do(func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) {
							Expect(obj).To(gomega.DeepDerivativeEqual(deploymentFor(version, config)))
						}),
					)

					Expect(kubeScheduler.Deploy(ctx)).To(Succeed())
				},

				Entry("kubernetes 1.10 w/o config", "1.10.0", configEmpty),
				Entry("kubernetes 1.10 w/ full config", "1.10.0", configFull),
				Entry("kubernetes 1.11 w/o config", "1.11.1", configEmpty),
				Entry("kubernetes 1.11 w/ full config", "1.11.1", configFull),
				Entry("kubernetes 1.12 w/o config", "1.12.2", configEmpty),
				Entry("kubernetes 1.12 w/ full config", "1.12.2", configFull),
				Entry("kubernetes 1.13 w/o config", "1.13.3", configEmpty),
				Entry("kubernetes 1.13 w/ full config", "1.13.3", configFull),
				Entry("kubernetes 1.14 w/o config", "1.14.4", configEmpty),
				Entry("kubernetes 1.14 w/ full config", "1.14.4", configFull),
				Entry("kubernetes 1.15 w/o config", "1.15.5", configEmpty),
				Entry("kubernetes 1.15 w/ full config", "1.15.5", configFull),
				Entry("kubernetes 1.16 w/o config", "1.16.6", configEmpty),
				Entry("kubernetes 1.16 w/ full config", "1.16.6", configFull),
				Entry("kubernetes 1.17 w/o config", "1.17.7", configEmpty),
				Entry("kubernetes 1.17 w/ full config", "1.17.7", configFull),
				Entry("kubernetes 1.18 w/o config", "1.18.8", configEmpty),
				Entry("kubernetes 1.18 w/ full config", "1.18.8", configFull),
				Entry("kubernetes 1.19 w/o config", "1.19.8", configEmpty),
				Entry("kubernetes 1.19 w/ full config", "1.19.8", configFull),
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

func componentConfigYAMLForKubernetesVersion(version string) (string, string) {
	var apiVersion, checksum string
	if k8sVersionGreaterEqual119, _ := versionutils.CompareVersions(version, ">=", "1.19"); k8sVersionGreaterEqual119 {
		apiVersion, checksum = "kubescheduler.config.k8s.io/v1beta1", "9988c880500b124fb153fd6e8c34435386b1924dcd48a39385fdb6d2bef492a9"
	} else if k8sVersionGreaterEqual118, _ := versionutils.CompareVersions(version, ">=", "1.18"); k8sVersionGreaterEqual118 {
		apiVersion, checksum = "kubescheduler.config.k8s.io/v1alpha2", "a1916d3e007de7f094bb829768c16eef1c4ec2ba30087e3bb1e564ecd2990fc5"
	} else if k8sVersionGreaterEqual112, _ := versionutils.CompareVersions(version, ">=", "1.12"); k8sVersionGreaterEqual112 {
		apiVersion, checksum = "kubescheduler.config.k8s.io/v1alpha1", "b1821b1b8e76431815e36877e29eda9a50fbdd9adc2a9e579f378fe69a6f744c"
	} else {
		apiVersion, checksum = "componentconfig/v1alpha1", "a09f1f2d75393187666e68e29d3f405db81f8027d1111918ce40e4cb31c2e696"
	}

	return `apiVersion: ` + apiVersion + `
kind: KubeSchedulerConfiguration
clientConnection:
  kubeconfig: /var/lib/kube-scheduler/kubeconfig
leaderElection:
  leaderElect: true`, checksum
}

func portForKubernetesVersion(version string) int32 {
	if k8sVersionGreaterEqual113, _ := versionutils.CompareVersions(version, ">=", "1.13"); k8sVersionGreaterEqual113 {
		return 10259
	}
	return 10251
}

func probeSchemeForKubernetesVersion(version string) corev1.URIScheme {
	if k8sVersionGreaterEqual113, _ := versionutils.CompareVersions(version, ">=", "1.13"); k8sVersionGreaterEqual113 {
		return corev1.URISchemeHTTPS
	}
	return corev1.URISchemeHTTP
}

func commandForKubernetesVersion(version string, port int32, featureGateFlags ...string) []string {
	var command []string

	if k8sVersionLessThan115, _ := versionutils.CompareVersions(version, "<", "1.15"); k8sVersionLessThan115 {
		command = append(command, "/hyperkube", "scheduler")
	} else if k8sVersionLessThan117, _ := versionutils.CompareVersions(version, "<", "1.17"); k8sVersionLessThan117 {
		command = append(command, "/hyperkube", "kube-scheduler")
	} else {
		command = append(command, "/usr/local/bin/kube-scheduler")
	}

	command = append(command, "--config=/var/lib/kube-scheduler-config/config.yaml")

	if k8sVersionGreaterEqual113, _ := versionutils.CompareVersions(version, ">=", "1.13"); k8sVersionGreaterEqual113 {
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
	}

	command = append(command, featureGateFlags...)
	command = append(command, "--v=2")

	return command
}

func featureGateFlagsForKubernetesVersion(version string, config *gardencorev1beta1.KubeSchedulerConfig) []string {
	var out []string

	if config != nil && config.FeatureGates != nil {
		out = append(out, kutil.FeatureGatesToCommandLineParameter(config.FeatureGates))
	}
	if k8sVersionLessThan111, _ := versionutils.CompareVersions(version, "<", "1.11"); k8sVersionLessThan111 {
		out = append(out, kutil.FeatureGatesToCommandLineParameter(map[string]bool{"PodPriority": true}))
	}

	return out
}
