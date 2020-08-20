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

package controlplane_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientset "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/operation/botanist/controlplane"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
	. "github.com/gardener/gardener/test/gomega"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	apimachineryversion "k8s.io/apimachinery/pkg/version"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/utils/pointer"
)

var _ = Describe("KubeScheduler", func() {
	var (
		applier       *fakeclientset.Applier
		chartApplier  kubernetes.ChartApplier
		kubeScheduler KubeScheduler

		err       error
		ctx       = context.TODO()
		namespace = "shoot--foo--bar"
		version   = "1.17.2"
		images    = map[string]*imagevector.Image{
			common.KubeSchedulerImageName: {
				Repository: "k8s.gcr.io/kube-scheduler",
				Tag:        pointer.StringPtr(version),
			}}
		replicas int32 = 1

		configEmpty *gardencorev1beta1.KubeSchedulerConfig
		configFull  = &gardencorev1beta1.KubeSchedulerConfig{KubernetesConfig: gardencorev1beta1.KubernetesConfig{FeatureGates: map[string]bool{"Foo": true, "Bar": false, "Baz": false}}, KubeMaxPDVols: pointer.StringPtr("23")}

		configMapName            = "kube-scheduler-config"
		secretNameKubeconfig     = "kube-scheduler"
		secretChecksumKubeconfig = "1234"
		secretNameServer         = "kube-scheduler-server"
		secretChecksumServer     = "5678"

		volumeNameConfig     = "kube-scheduler-config"
		volumeNameKubeconfig = "kubeconfig-secret"
		volumeNameServer     = "server-secret"
		vpaName              = "kube-scheduler-vpa"
		vpaUpdateMode        = autoscalingv1beta2.UpdateModeAuto
		serviceName          = "kube-scheduler"
		deploymentName       = "kube-scheduler"

		labels = map[string]string{
			"app":                     "kubernetes",
			"role":                    "scheduler",
			"garden.sapcloud.io/role": "controlplane",
		}

		controlPlaneChartsPath = filepath.Join(chartsRoot(), "seed-controlplane", "charts")
		objectIdentifier       Identifier

		configMapFor = func(version string) *corev1.ConfigMap {
			componentConfigYAML, _ := componentConfigYAMLForKubernetesVersion(version)
			return &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName,
					Namespace: namespace,
					Labels:    labels,
				},
				Data: map[string]string{"config.yaml": componentConfigYAML},
			}
		}
		vpa = &autoscalingv1beta2.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      vpaName,
				Namespace: namespace,
				Labels:    labels,
			},
			Spec: autoscalingv1beta2.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       "kube-scheduler",
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
					Labels:    labels,
				},
				Spec: corev1.ServiceSpec{
					ClusterIP: corev1.ClusterIPNone,
					Selector:  labels,
					Type:      corev1.ServiceTypeClusterIP,
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
					Labels:    labels,
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: pointer.Int32Ptr(0),
					Replicas:             &replicas,
					Selector: &metav1.LabelSelector{
						MatchLabels: labels,
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								"checksum/configmap-componentconfig":      componentConfigChecksum,
								"checksum/secret-" + secretNameKubeconfig: secretChecksumKubeconfig,
								"checksum/secret-" + secretNameServer:     secretChecksumServer,
							},
							Labels: utils.MergeStringMaps(labels, map[string]string{
								"networking.gardener.cloud/to-dns":             "allowed",
								"networking.gardener.cloud/to-shoot-apiserver": "allowed",
								"networking.gardener.cloud/from-prometheus":    "allowed",
							}),
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:            "kube-scheduler",
									Image:           images[common.KubeSchedulerImageName].String(),
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
									Env:                      env,
									TerminationMessagePath:   "/dev/termination-log",
									TerminationMessagePolicy: corev1.TerminationMessageReadFile,
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
											Name:      volumeNameKubeconfig,
											MountPath: "/var/lib/kube-scheduler",
										},
										{
											Name:      volumeNameServer,
											MountPath: "/var/lib/kube-scheduler-server",
										},
										{
											Name:      volumeNameConfig,
											MountPath: "/var/lib/kube-scheduler-config",
										},
									},
								},
							},
							DNSPolicy:                     corev1.DNSClusterFirst,
							RestartPolicy:                 corev1.RestartPolicyAlways,
							SchedulerName:                 corev1.DefaultSchedulerName,
							TerminationGracePeriodSeconds: pointer.Int64Ptr(30),
							Volumes: []corev1.Volume{
								{
									Name: volumeNameKubeconfig,
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: secretNameKubeconfig,
										},
									},
								},
								{
									Name: volumeNameServer,
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName: secretNameServer,
										},
									},
								},
								{
									Name: volumeNameConfig,
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
		format.CharactersAroundMismatchToInclude = 30

		scheme := kubernetes.SeedScheme
		applier = fakeclientset.NewApplier(scheme)
		chartApplier = kubernetes.NewChartApplier(chartrenderer.NewWithServerVersion(&apimachineryversion.Info{}), applier)

		objectIdentifier = ObjectIdentifierForScheme(scheme)

		kubeScheduler, err = NewKubeScheduler(
			chartApplier,
			controlPlaneChartsPath,
			namespace,
			version,
			images,
			replicas,
			configEmpty,
		)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("#Deploy", func() {
		BeforeEach(func() {
			kubeScheduler.SetSecrets(KubeSchedulerSecrets{
				Kubeconfig: Secret{Name: secretNameKubeconfig, Checksum: secretChecksumKubeconfig},
				Server:     Secret{Name: secretNameServer, Checksum: secretChecksumServer},
			})
		})

		DescribeTable("success tests for various kubernetes versions",
			func(version string, config *gardencorev1beta1.KubeSchedulerConfig) {
				kubeScheduler, err = NewKubeScheduler(
					chartApplier,
					controlPlaneChartsPath,
					namespace,
					version,
					images,
					replicas,
					config,
				)
				Expect(err).NotTo(HaveOccurred())

				kubeScheduler.SetSecrets(KubeSchedulerSecrets{
					Kubeconfig: Secret{Name: secretNameKubeconfig, Checksum: secretChecksumKubeconfig},
					Server:     Secret{Name: secretNameServer, Checksum: secretChecksumServer},
				})

				configMap, service, deployment := configMapFor(version), serviceFor(version), deploymentFor(version, config)

				Expect(kubeScheduler.Deploy(ctx)).To(Succeed())
				Expect(applier.Objects).To(MatchAllElements(objectIdentifier, Elements{
					objectIdentifier(configMap):  DeepDerivativeEqual(configMap),
					objectIdentifier(vpa):        DeepDerivativeEqual(vpa),
					objectIdentifier(service):    DeepDerivativeEqual(service),
					objectIdentifier(deployment): DeepDerivativeEqual(deployment),
				}))
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
		)
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
	apiVersion, checksum := "componentconfig/v1alpha1", "8ed8fd137cea51744a2691f9e9dfce23281edb1bc23ff3f4fdcb5b6adc5a050a"

	if k8sVersionGreaterEqual118, _ := versionutils.CompareVersions(version, ">=", "1.18"); k8sVersionGreaterEqual118 {
		apiVersion, checksum = "kubescheduler.config.k8s.io/v1alpha2", "83fcf613e713f43588bf184a2d2165d2c0044b05a517ef4783c7d6087f1a76b8"
	} else if k8sVersionGreaterEqual112, _ := versionutils.CompareVersions(version, ">=", "1.12"); k8sVersionGreaterEqual112 {
		apiVersion, checksum = "kubescheduler.config.k8s.io/v1alpha1", "7bfc126dc5bbf47156bce896413ba651946f6ceaa40b78d2b3ed554ac5799c3d"
	}

	return "---\napiVersion: " + apiVersion + "\nkind: KubeSchedulerConfiguration\nclientConnection:\n  kubeconfig: /var/lib/kube-scheduler/kubeconfig\nleaderElection:\n  leaderElect: true", checksum
}

func portForKubernetesVersion(version string) int32 {
	if k8sVersionGreateEqual113, _ := versionutils.CompareVersions(version, ">=", "1.13"); k8sVersionGreateEqual113 {
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

	if len(out) > 0 {
		out[len(out)-1] = strings.TrimSuffix(out[len(out)-1], ",")
	}

	return out
}

func ObjectIdentifierForScheme(scheme *runtime.Scheme) Identifier {
	return func(element interface{}) string {
		obj, ok := element.(kutil.Object)
		if !ok {
			Fail("element does not implement the Object interfaces")
		}

		gvks, _, err := scheme.ObjectKinds(obj)
		if err != nil {
			Fail(err.Error())
		}

		return fmt.Sprintf("%s/%s/%s/%s", gvks[0].GroupVersion().String(), gvks[0].Kind, obj.GetNamespace(), obj.GetName())
	}
}
