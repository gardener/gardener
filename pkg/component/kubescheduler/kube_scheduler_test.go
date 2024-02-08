// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"os"
	"strconv"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/kubescheduler"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

var _ = Describe("KubeScheduler", func() {
	var (
		c                             client.Client
		sm                            secretsmanager.Interface
		kubeScheduler                 Interface
		ctx                           = context.TODO()
		namespace                     = "shoot--foo--bar"
		runtimeVersion, targetVersion *semver.Version
		image                               = "registry.k8s.io/kube-scheduler:v1.27.2"
		replicas                      int32 = 1
		profileBinPacking                   = gardencorev1beta1.SchedulingProfileBinPacking
		configEmpty                   *gardencorev1beta1.KubeSchedulerConfig
		configFull                    = &gardencorev1beta1.KubeSchedulerConfig{
			KubernetesConfig: gardencorev1beta1.KubernetesConfig{
				FeatureGates: map[string]bool{"Foo": true, "Bar": false, "Baz": false},
			},
			KubeMaxPDVols: ptr.To("23"),
			Profile:       &profileBinPacking,
		}

		secretNameClientCA = "ca-client"
		secretNameServer   = "kube-scheduler-server"

		genericTokenKubeconfigSecretName = "generic-token-kubeconfig"
		vpaName                          = "kube-scheduler-vpa"
		pdbName                          = "kube-scheduler"
		serviceName                      = "kube-scheduler"
		secretName                       = "shoot-access-kube-scheduler"
		deploymentName                   = "kube-scheduler"
		managedResourceName              = "shoot-core-kube-scheduler"
		managedResourceSecretName        = "managedresource-shoot-core-kube-scheduler"

		configMapFor = func(componentConfigFilePath string) *corev1.ConfigMap {
			data, err := os.ReadFile(componentConfigFilePath)
			Expect(err).NotTo(HaveOccurred())
			componentConfigYAML := string(data)

			cm := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-scheduler-config", Namespace: namespace},
				Data:       map[string]string{"config.yaml": componentConfigYAML},
			}
			Expect(kubernetesutils.MakeUnique(cm)).To(Succeed())
			return cm
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      "kube-scheduler",
					"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
				},
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
					"resources.gardener.cloud/class":   "shoot",
				},
				ResourceVersion: "0",
			},
			Type: corev1.SecretTypeOpaque,
		}

		pdbMaxUnavailable = intstr.FromInt32(1)
		pdbFor            = func(runtimeVersion *semver.Version) *policyv1.PodDisruptionBudget {
			pdb := &policyv1.PodDisruptionBudget{
				TypeMeta: metav1.TypeMeta{
					APIVersion: policyv1.SchemeGroupVersion.String(),
					Kind:       "PodDisruptionBudget",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      pdbName,
					Namespace: namespace,
					Labels: map[string]string{
						"app":  "kubernetes",
						"role": "scheduler",
					},
					ResourceVersion: "1",
				},
				Spec: policyv1.PodDisruptionBudgetSpec{
					MaxUnavailable: &pdbMaxUnavailable,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":  "kubernetes",
							"role": "scheduler",
						},
					},
				},
			}

			unhealthyPodEvictionPolicyAlwatysAllow := policyv1.AlwaysAllow
			if versionutils.ConstraintK8sGreaterEqual126.Check(runtimeVersion) {
				pdb.Spec.UnhealthyPodEvictionPolicy = &unhealthyPodEvictionPolicyAlwatysAllow
			}

			return pdb
		}

		vpaUpdateMode    = vpaautoscalingv1.UpdateModeAuto
		controlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		vpa              = &vpaautoscalingv1.VerticalPodAutoscaler{
			TypeMeta: metav1.TypeMeta{
				APIVersion: vpaautoscalingv1.SchemeGroupVersion.String(),
				Kind:       "VerticalPodAutoscaler",
			},
			ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace, ResourceVersion: "1"},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Name:       deploymentName,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("50Mi"),
							},
							MaxAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4"),
								corev1.ResourceMemory: resource.MustParse("10G"),
							},
							ControlledValues: &controlledValues,
						},
					},
				},
			},
		}
		service = &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Service",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "scheduler",
				},
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":10259}]`,
				},
				ResourceVersion: "1",
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
		deploymentFor = func(config *gardencorev1beta1.KubeSchedulerConfig, componentConfigFilePath string) *appsv1.Deployment {
			var env []corev1.EnvVar
			if config != nil && config.KubeMaxPDVols != nil {
				env = append(env, corev1.EnvVar{
					Name:  "KUBE_MAX_PD_VOLS",
					Value: *config.KubeMaxPDVols,
				})
			}

			configMap := configMapFor(componentConfigFilePath)

			deploy := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: namespace,
					Labels: map[string]string{
						"app":                 "kubernetes",
						"role":                "scheduler",
						"gardener.cloud/role": "controlplane",
						"high-availability-config.resources.gardener.cloud/type": "controller",
					},
					ResourceVersion: "1",
				},
				Spec: appsv1.DeploymentSpec{
					RevisionHistoryLimit: ptr.To(int32(1)),
					Replicas:             &replicas,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app":  "kubernetes",
							"role": "scheduler",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app":                                "kubernetes",
								"role":                               "scheduler",
								"gardener.cloud/role":                "controlplane",
								"maintenance.gardener.cloud/restart": "true",
								"networking.gardener.cloud/to-dns":   "allowed",
								"networking.resources.gardener.cloud/to-kube-apiserver-tcp-443": "allowed",
							},
						},
						Spec: corev1.PodSpec{
							AutomountServiceAccountToken: ptr.To(false),
							SecurityContext: &corev1.PodSecurityContext{
								RunAsNonRoot: ptr.To(true),
								RunAsUser:    ptr.To(int64(65532)),
								RunAsGroup:   ptr.To(int64(65532)),
								FSGroup:      ptr.To(int64(65532)),
							},
							Containers: []corev1.Container{
								{
									Name:            "kube-scheduler",
									Image:           image,
									ImagePullPolicy: corev1.PullIfNotPresent,
									Command:         commandForKubernetesVersion(10259, featureGateFlags(config)...),
									LivenessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path:   "/healthz",
												Scheme: corev1.URISchemeHTTPS,
												Port:   intstr.FromInt32(10259),
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
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "client-ca",
											MountPath: "/var/lib/kube-scheduler-client-ca",
										},
										{
											Name:      "kube-scheduler-server",
											MountPath: "/var/lib/kube-scheduler-server",
										},
										{
											Name:      "kube-scheduler-config",
											MountPath: "/var/lib/kube-scheduler-config",
										},
									},
								},
							},
							PriorityClassName: v1beta1constants.PriorityClassNameShootControlPlane300,
							Volumes: []corev1.Volume{
								{
									Name: "client-ca",
									VolumeSource: corev1.VolumeSource{
										Projected: &corev1.ProjectedVolumeSource{
											DefaultMode: ptr.To(int32(420)),
											Sources: []corev1.VolumeProjection{
												{
													Secret: &corev1.SecretProjection{
														LocalObjectReference: corev1.LocalObjectReference{
															Name: secretNameClientCA,
														},
														Items: []corev1.KeyToPath{{
															Key:  "bundle.crt",
															Path: "bundle.crt",
														}},
													},
												},
											},
										},
									},
								},
								{
									Name: "kube-scheduler-server",
									VolumeSource: corev1.VolumeSource{
										Secret: &corev1.SecretVolumeSource{
											SecretName:  secretNameServer,
											DefaultMode: ptr.To(int32(0640)),
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

			Expect(gardenerutils.InjectGenericKubeconfig(deploy, genericTokenKubeconfigSecretName, secret.Name)).To(Succeed())
			Expect(references.InjectAnnotations(deploy)).To(Succeed())
			return deploy
		}
		clusterRoleBinding1YAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: gardener.cloud:target:kube-scheduler
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:kube-scheduler
subjects:
- kind: ServiceAccount
  name: kube-scheduler
  namespace: kube-system
`
		clusterRoleBinding2YAML = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: gardener.cloud:target:kube-scheduler-volume
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:volume-scheduler
subjects:
- kind: ServiceAccount
  name: kube-scheduler
  namespace: kube-system
`
		managedResourceSecret *corev1.Secret
		managedResource       *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		sm = fakesecretsmanager.New(c, namespace)
		targetVersion = semver.MustParse("1.27.2")
		runtimeVersion = semver.MustParse("1.25.2")
		kubeScheduler = New(c, namespace, sm, runtimeVersion, targetVersion, image, replicas, configEmpty)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-client", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
	})

	Describe("#Deploy", func() {
		BeforeEach(func() {
			managedResourceSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceSecretName,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
			}
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceName,
					Namespace: namespace,
					Labels:    map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs: []corev1.LocalObjectReference{
						{Name: managedResourceSecretName},
					},
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					KeepObjects:  ptr.To(false),
				},
			}
		})

		DescribeTable("success tests for various kubernetes versions",
			func(targetVersion, runtimeVersion string, config *gardencorev1beta1.KubeSchedulerConfig, expectedComponentConfigFilePath string) {
				targetSemverVersion, err := semver.NewVersion(targetVersion)
				Expect(err).NotTo(HaveOccurred())

				runtimeSemverVersion, err := semver.NewVersion(runtimeVersion)
				Expect(err).NotTo(HaveOccurred())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, managedResource.Name)))

				kubeScheduler = New(c, namespace, sm, runtimeSemverVersion, targetSemverVersion, image, replicas, config)
				Expect(kubeScheduler.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				expectedMr := &resourcesv1alpha1.ManagedResource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ManagedResource",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResource.Name,
						Namespace:       managedResource.Namespace,
						ResourceVersion: "1",
						Labels:          map[string]string{"origin": "gardener"},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
						SecretRefs: []corev1.LocalObjectReference{{
							Name: managedResource.Spec.SecretRefs[0].Name,
						}},
						KeepObjects: ptr.To(false),
					},
				}
				utilruntime.Must(references.InjectAnnotations(expectedMr))
				Expect(managedResource).To(DeepEqual(expectedMr))

				managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
				Expect(managedResourceSecret.Data).To(HaveLen(2))
				Expect(managedResourceSecret.Data["clusterrolebinding____gardener.cloud_target_kube-scheduler.yaml"]).To(Equal([]byte(clusterRoleBinding1YAML)))
				Expect(managedResourceSecret.Data["clusterrolebinding____gardener.cloud_target_kube-scheduler-volume.yaml"]).To(Equal([]byte(clusterRoleBinding2YAML)))

				actualDeployment := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      deploymentName,
						Namespace: namespace,
					},
				}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(actualDeployment), actualDeployment)).To(Succeed())
				Expect(actualDeployment).To(DeepEqual(deploymentFor(config, expectedComponentConfigFilePath)))

				actualVPA := &vpaautoscalingv1.VerticalPodAutoscaler{
					ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace},
				}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(actualVPA), actualVPA)).To(Succeed())
				Expect(actualVPA).To(DeepEqual(vpa))

				actualService := &corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      serviceName,
						Namespace: namespace,
					},
				}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(actualService), actualService)).To(Succeed())
				Expect(actualService).To(DeepEqual(service))

				actualPDB := &policyv1.PodDisruptionBudget{
					ObjectMeta: metav1.ObjectMeta{
						Name:      pdbName,
						Namespace: namespace,
					},
				}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(actualPDB), actualPDB)).To(Succeed())
				Expect(actualPDB).To(DeepEqual(pdbFor(runtimeSemverVersion)))
			},

			Entry("kubernetes 1.24 w/o config", "1.24.1", "1.24.1", configEmpty, "testdata/component-config-1.24.yaml"),
			Entry("kubernetes 1.24 w/ full config", "1.24.1", "1.24.1", configFull, "testdata/component-config-1.24-bin-packing.yaml"),
			Entry("kubernetes 1.25 w/o config", "1.25.0", "1.25.0", configEmpty, "testdata/component-config-1.25.yaml"),
			Entry("kubernetes 1.25 w/ full config", "1.25.0", "1.25.0", configFull, "testdata/component-config-1.25-bin-packing.yaml"),
			Entry("kubernetes 1.26 w/o config", "1.26.0", "1.26.0", configEmpty, "testdata/component-config-1.25.yaml"),
			Entry("kubernetes 1.26 w/ full config", "1.26.0", "1.26.0", configFull, "testdata/component-config-1.25-bin-packing.yaml"),
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

func commandForKubernetesVersion(port int32, featureGateFlags ...string) []string {
	var command []string

	command = append(command,
		"/usr/local/bin/kube-scheduler",
		"--config=/var/lib/kube-scheduler-config/config.yaml",
		"--authentication-kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
		"--authorization-kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
		"--client-ca-file=/var/lib/kube-scheduler-client-ca/bundle.crt",
		"--tls-cert-file=/var/lib/kube-scheduler-server/tls.crt",
		"--tls-private-key-file=/var/lib/kube-scheduler-server/tls.key",
		"--secure-port="+strconv.Itoa(int(port)),
	)

	command = append(command, featureGateFlags...)
	command = append(command, "--v=2")

	return command
}

func featureGateFlags(config *gardencorev1beta1.KubeSchedulerConfig) []string {
	var out []string

	if config != nil && config.FeatureGates != nil {
		out = append(out, kubernetesutils.FeatureGatesToCommandLineParameter(config.FeatureGates))
	}

	return out
}
