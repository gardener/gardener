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
	"fmt"
	"os"
	"strconv"

	"github.com/Masterminds/semver"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/kubescheduler"
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
		ctrl                     *gomock.Controller
		c                        *mockclient.MockClient
		fakeClient               client.Client
		sm                       secretsmanager.Interface
		kubeScheduler            Interface
		ctx                            = context.TODO()
		fakeErr                        = fmt.Errorf("fake error")
		namespace                      = "shoot--foo--bar"
		version                        = "1.23.2"
		semverVersion, _               = semver.NewVersion(version)
		image                          = "registry.k8s.io/kube-scheduler:v1.23.2"
		replicas                 int32 = 1
		profileBinPacking              = gardencorev1beta1.SchedulingProfileBinPacking
		runtimeKubernetesVersion       = semver.MustParse("1.25.0")
		configEmpty              *gardencorev1beta1.KubeSchedulerConfig
		configFull               = &gardencorev1beta1.KubeSchedulerConfig{
			KubernetesConfig: gardencorev1beta1.KubernetesConfig{
				FeatureGates: map[string]bool{"Foo": true, "Bar": false, "Baz": false},
			},
			KubeMaxPDVols: pointer.String("23"),
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
				},
			},
			Type: corev1.SecretTypeOpaque,
		}

		pdbMaxUnavailable = intstr.FromInt(1)
		pdb               = &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pdbName,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "scheduler",
				},
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

		vpaUpdateMode    = vpaautoscalingv1.UpdateModeAuto
		controlledValues = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		vpa              = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{Name: vpaName, Namespace: namespace},
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
		serviceFor = func(version string) *corev1.Service {
			return &corev1.Service{
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
		deploymentFor = func(version string, config *gardencorev1beta1.KubeSchedulerConfig, componentConfigFilePath string) *appsv1.Deployment {
			var env []corev1.EnvVar
			if config != nil && config.KubeMaxPDVols != nil {
				env = append(env, corev1.EnvVar{
					Name:  "KUBE_MAX_PD_VOLS",
					Value: *config.KubeMaxPDVols,
				})
			}

			configMap := configMapFor(componentConfigFilePath)

			deploy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: namespace,
					Labels: map[string]string{
						"app":                 "kubernetes",
						"role":                "scheduler",
						"gardener.cloud/role": "controlplane",
						"high-availability-config.resources.gardener.cloud/type": "controller",
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
							AutomountServiceAccountToken: pointer.Bool(false),
							Containers: []corev1.Container{
								{
									Name:            "kube-scheduler",
									Image:           image,
									ImagePullPolicy: corev1.PullIfNotPresent,
									Command:         commandForKubernetesVersion(version, 10259, featureGateFlags(config)...),
									LivenessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
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
											DefaultMode: pointer.Int32(420),
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
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceSecretName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"clusterrolebinding____gardener.cloud_target_kube-scheduler.yaml":        []byte(clusterRoleBinding1YAML),
				"clusterrolebinding____gardener.cloud_target_kube-scheduler-volume.yaml": []byte(clusterRoleBinding2YAML),
			},
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
				KeepObjects:  pointer.Bool(false),
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		sm = fakesecretsmanager.New(fakeClient, namespace)
		kubeScheduler = New(c, namespace, sm, semverVersion, image, replicas, configEmpty, runtimeKubernetesVersion)

		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-client", Namespace: namespace}})).To(Succeed())
		Expect(fakeClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		It("should fail because the configmap cannot be created", func() {
			gomock.InOrder(
				c.EXPECT().Create(ctx, configMapFor("testdata/component-config-1.23.yaml")).Return(fakeErr),
			)

			Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail when the service cannot be created", func() {
			gomock.InOrder(
				c.EXPECT().Create(ctx, configMapFor("testdata/component-config-1.23.yaml")),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).Return(fakeErr),
			)

			Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail when the secret cannot be created", func() {
			gomock.InOrder(
				c.EXPECT().Create(ctx, configMapFor("testdata/component-config-1.23.yaml")),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, secret.Name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).Return(fakeErr),
			)

			Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the deployment cannot be created", func() {
			gomock.InOrder(
				c.EXPECT().Create(ctx, configMapFor("testdata/component-config-1.23.yaml")),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, secret.Name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).Return(fakeErr),
			)

			Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the pod disruption budget cannot be created", func() {
			gomock.InOrder(
				c.EXPECT().Create(ctx, configMapFor("testdata/component-config-1.23.yaml")),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, secret.Name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, pdbName), gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()).Return(fakeErr),
			)

			Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the vpa cannot be created", func() {
			gomock.InOrder(
				c.EXPECT().Create(ctx, configMapFor("testdata/component-config-1.23.yaml")),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, secret.Name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, pdbName), gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, vpaName), gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).Return(fakeErr),
			)

			Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the managed resource cannot be created", func() {
			gomock.InOrder(
				c.EXPECT().Create(ctx, configMapFor("testdata/component-config-1.23.yaml")),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, secret.Name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, pdbName), gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, vpaName), gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).Return(fakeErr),
			)

			Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the managed resource secret cannot be created", func() {
			gomock.InOrder(
				c.EXPECT().Create(ctx, configMapFor("testdata/component-config-1.23.yaml")),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, secret.Name), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, pdbName), gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, vpaName), gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{}), gomock.Any()).Return(fakeErr),
			)

			Expect(kubeScheduler.Deploy(ctx)).To(MatchError(fakeErr))
		})

		DescribeTable("success tests for various kubernetes versions",
			func(version string, config *gardencorev1beta1.KubeSchedulerConfig, expectedComponentConfigFilePath string) {
				semverVersion, err := semver.NewVersion(version)
				Expect(err).NotTo(HaveOccurred())

				kubeScheduler = New(c, namespace, sm, semverVersion, image, replicas, config, runtimeKubernetesVersion)

				gomock.InOrder(
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.ConfigMap{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ ...client.CreateOption) {
							Expect(obj).To(DeepEqual(configMapFor(expectedComponentConfigFilePath)))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, serviceName), gomock.AssignableToTypeOf(&corev1.Service{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Service{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(serviceFor(version)))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(secret))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, deploymentName), gomock.AssignableToTypeOf(&appsv1.Deployment{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&appsv1.Deployment{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(deploymentFor(version, config, expectedComponentConfigFilePath)))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, pdbName), gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&policyv1.PodDisruptionBudget{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(pdb))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, vpaName), gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&vpaautoscalingv1.VerticalPodAutoscaler{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(vpa))
						}),

					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).
						Do(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) {
							Expect(obj).To(DeepEqual(managedResourceSecret))
						}),
					c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).
						Do(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) {
							Expect(obj).To(DeepEqual(managedResource))
						}),
				)

				Expect(kubeScheduler.Deploy(ctx)).To(Succeed())
			},

			Entry("kubernetes 1.20 w/o config", "1.20.9", configEmpty, "testdata/component-config-1.20.yaml"),
			Entry("kubernetes 1.20 w/ full config", "1.20.9", configFull, "testdata/component-config-1.20-bin-packing.yaml"),
			Entry("kubernetes 1.21 w/o config", "1.21.3", configEmpty, "testdata/component-config-1.20.yaml"),
			Entry("kubernetes 1.21 w/ full config", "1.21.3", configFull, "testdata/component-config-1.20-bin-packing.yaml"),
			Entry("kubernetes 1.22 w/o config", "1.22.1", configEmpty, "testdata/component-config-1.22.yaml"),
			Entry("kubernetes 1.22 w/ full config", "1.22.1", configFull, "testdata/component-config-1.22-bin-packing.yaml"),
			Entry("kubernetes 1.23 w/o config", "1.23.1", configEmpty, "testdata/component-config-1.23.yaml"),
			Entry("kubernetes 1.23 w/ full config", "1.23.1", configFull, "testdata/component-config-1.23-bin-packing.yaml"),
			Entry("kubernetes 1.24 w/o config", "1.24.1", configEmpty, "testdata/component-config-1.23.yaml"),
			Entry("kubernetes 1.24 w/ full config", "1.24.1", configFull, "testdata/component-config-1.23-bin-packing.yaml"),
			Entry("kubernetes 1.25 w/o config", "1.25.0", configEmpty, "testdata/component-config-1.25.yaml"),
			Entry("kubernetes 1.25 w/ full config", "1.25.0", configFull, "testdata/component-config-1.25-bin-packing.yaml"),
			Entry("kubernetes 1.26 w/o config", "1.26.0", configEmpty, "testdata/component-config-1.25.yaml"),
			Entry("kubernetes 1.26 w/ full config", "1.26.0", configFull, "testdata/component-config-1.25-bin-packing.yaml"),
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

func commandForKubernetesVersion(version string, port int32, featureGateFlags ...string) []string {
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

	if k8sVersionLessThan123, _ := versionutils.CompareVersions(version, "<", "1.23"); k8sVersionLessThan123 {
		command = append(command, "--port=0")
	}

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
