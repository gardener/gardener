// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package prometheus_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/monitoring/prometheus"
	componenttest "github.com/gardener/gardener/pkg/component/test"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Prometheus", func() {
	var (
		ctx context.Context

		name                = "test"
		namespace           = "some-namespace"
		managedResourceName = "prometheus-" + name

		image             = "some-image"
		version           = "v1.2.3"
		priorityClassName = "priority-class"
		storageCapacity   = resource.MustParse("1337Gi")

		additionalScrapeConfig1 = `job_name: foo
honor_labels: false`
		additionalScrapeConfig2 = `job_name: bar
honor_labels: true`

		fakeClient client.Client
		deployer   component.DeployWaiter
		values     Values

		fakeOps *retryfake.Ops

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		reloadStrategy = monitoringv1.HTTPReloadStrategyType

		serviceAccount                *corev1.ServiceAccount
		service                       *corev1.Service
		clusterRoleBinding            *rbacv1.ClusterRoleBinding
		prometheus                    *monitoringv1.Prometheus
		vpa                           *vpaautoscalingv1.VerticalPodAutoscaler
		prometheusRule                *monitoringv1.PrometheusRule
		serviceMonitor                *monitoringv1.ServiceMonitor
		additionalConfigMap           *corev1.ConfigMap
		secretAdditionalScrapeConfigs *corev1.Secret
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		values = Values{
			Name:              name,
			Image:             image,
			Version:           version,
			PriorityClassName: priorityClassName,
			StorageCapacity:   storageCapacity,
		}

		fakeOps = &retryfake.Ops{MaxAttempts: 2}
		DeferCleanup(test.WithVars(
			&retry.Until, fakeOps.Until,
			&retry.UntilTimeout, fakeOps.UntilTimeout,
		))

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: namespace,
			},
		}

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-" + name,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				},
			},
			AutomountServiceAccountToken: ptr.To(false),
		}
		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-" + name,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				},
				Annotations: map[string]string{
					"networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports": `[{"protocol":"TCP","port":9090}]`,
					"networking.resources.gardener.cloud/namespace-selectors":                        `[{"matchLabels":{"gardener.cloud/role":"shoot"}}]`,
				},
			},
			Spec: corev1.ServiceSpec{
				Type:     corev1.ServiceTypeClusterIP,
				Selector: map[string]string{"prometheus": name},
				Ports: []corev1.ServicePort{{
					Name:       "web",
					Port:       80,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt32(9090),
				}},
			},
		}
		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "prometheus-" + name,
				Labels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "prometheus",
			},
			Subjects: []rbacv1.Subject{{
				Kind:      "ServiceAccount",
				Name:      "prometheus-" + name,
				Namespace: namespace,
			}},
		}
		prometheus = &monitoringv1.Prometheus{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				},
			},
			Spec: monitoringv1.PrometheusSpec{
				Retention:          "1d",
				RetentionSize:      "5GB",
				EvaluationInterval: "1m",
				CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
					ScrapeInterval: "1m",
					ReloadStrategy: &reloadStrategy,
					AdditionalScrapeConfigs: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "prometheus-" + name + "-additional-scrape-configs",
						},
						Key: "prometheus.yaml",
					},

					PodMetadata: &monitoringv1.EmbeddedObjectMetadata{
						Labels: map[string]string{
							"networking.gardener.cloud/to-dns":                               "allowed",
							"networking.gardener.cloud/to-runtime-apiserver":                 "allowed",
							"networking.resources.gardener.cloud/to-all-seed-scrape-targets": "allowed",
						},
					},
					PriorityClassName: priorityClassName,
					Replicas:          ptr.To(int32(1)),
					Image:             &image,
					ImagePullPolicy:   corev1.PullIfNotPresent,
					Version:           version,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("300m"),
							corev1.ResourceMemory: resource.MustParse("1000Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("2000Mi"),
						},
					},
					ServiceAccountName: "prometheus-" + name,
					SecurityContext:    &corev1.PodSecurityContext{RunAsUser: ptr.To(int64(0))},
					Storage: &monitoringv1.StorageSpec{
						VolumeClaimTemplate: monitoringv1.EmbeddedPersistentVolumeClaim{
							EmbeddedObjectMetadata: monitoringv1.EmbeddedObjectMetadata{Name: "prometheus-db"},
							Spec: corev1.PersistentVolumeClaimSpec{
								AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
								Resources:   corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: storageCapacity}},
							},
						},
					},

					ServiceMonitorSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"prometheus": name}},
					PodMonitorSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{"prometheus": name}},
					ProbeSelector:          &metav1.LabelSelector{MatchLabels: map[string]string{"prometheus": name}},
					ScrapeConfigSelector:   &metav1.LabelSelector{MatchLabels: map[string]string{"prometheus": name}},

					ServiceMonitorNamespaceSelector: &metav1.LabelSelector{},
					PodMonitorNamespaceSelector:     &metav1.LabelSelector{},
					ProbeNamespaceSelector:          &metav1.LabelSelector{},
					ScrapeConfigNamespaceSelector:   &metav1.LabelSelector{},
				},
				RuleSelector:          &metav1.LabelSelector{MatchLabels: map[string]string{"prometheus": name}},
				RuleNamespaceSelector: &metav1.LabelSelector{},
			},
		}
		vpaUpdateMode, vpaContainerScalingModeOff := vpaautoscalingv1.UpdateModeAuto, vpaautoscalingv1.ContainerScalingModeOff
		vpa = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-" + name,
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				},
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: "apps/v1",
					Kind:       "StatefulSet",
					Name:       "prometheus-" + name,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
						{
							ContainerName: "prometheus",
							MinAllowed: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("1000M"),
							},
							MaxAllowed: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("4"),
								corev1.ResourceMemory: resource.MustParse("28G"),
							},
						},
						{
							ContainerName: "config-reloader",
							Mode:          &vpaContainerScalingModeOff,
						},
					},
				},
			},
		}
		prometheusRule = &monitoringv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "rule",
				Labels: map[string]string{"foo": "bar"},
			},
			Spec: monitoringv1.PrometheusRuleSpec{
				Groups: []monitoringv1.RuleGroup{{Name: "foo"}},
			},
		}
		serviceMonitor = &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "monitor",
				Namespace: "default",
				Labels:    map[string]string{"foo": "bar"},
			},
			Spec: monitoringv1.ServiceMonitorSpec{
				JobLabel: "foo",
			},
		}
		additionalConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "configmap", Namespace: namespace},
		}
		secretAdditionalScrapeConfigs = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-" + name + "-additional-scrape-configs",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "prometheus",
					"role": "monitoring",
					"name": name,
				},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{"prometheus.yaml": []byte(`- job_name: foo
  honor_labels: false
- job_name: bar
  honor_labels: true
`)},
		}
	})

	JustBeforeEach(func() {
		values.AdditionalResources = append(values.AdditionalResources, additionalConfigMap)
		values.CentralConfigs.AdditionalScrapeConfigs = append(values.CentralConfigs.AdditionalScrapeConfigs, additionalScrapeConfig1, additionalScrapeConfig2)
		values.CentralConfigs.PrometheusRules = append(values.CentralConfigs.PrometheusRules, prometheusRule)
		values.CentralConfigs.ServiceMonitors = append(values.CentralConfigs.ServiceMonitors, serviceMonitor)

		deployer = New(logr.Discard(), fakeClient, namespace, values)
	})

	Describe("#Deploy", func() {
		Context("resources generation", func() {
			BeforeEach(func() {
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())

				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: healthyManagedResourceStatus,
				})).To(Succeed())
			})

			JustBeforeEach(func() {
				Expect(deployer.Deploy(ctx)).To(Succeed())

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
				expectedRuntimeMr := &resourcesv1alpha1.ManagedResource{
					TypeMeta: metav1.TypeMeta{
						APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ManagedResource",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            managedResource.Name,
						Namespace:       managedResource.Namespace,
						ResourceVersion: "2",
						Generation:      1,
						Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						Class:       ptr.To("seed"),
						SecretRefs:  []corev1.LocalObjectReference{{Name: managedResource.Spec.SecretRefs[0].Name}},
						KeepObjects: ptr.To(false),
					},
					Status: healthyManagedResourceStatus,
				}
				utilruntime.Must(references.InjectAnnotations(expectedRuntimeMr))
				Expect(managedResource).To(Equal(expectedRuntimeMr))

				managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

				Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
			})

			It("should successfully deploy all resources", func() {
				Expect(managedResourceSecret.Data).To(HaveLen(9))
				Expect(string(managedResourceSecret.Data["serviceaccount__some-namespace__prometheus-"+name+".yaml"])).To(Equal(componenttest.Serialize(serviceAccount)))
				Expect(string(managedResourceSecret.Data["service__some-namespace__prometheus-"+name+".yaml"])).To(Equal(componenttest.Serialize(service)))
				Expect(string(managedResourceSecret.Data["clusterrolebinding____prometheus-"+name+".yaml"])).To(Equal(componenttest.Serialize(clusterRoleBinding)))
				Expect(string(managedResourceSecret.Data["prometheus__some-namespace__"+name+".yaml"])).To(Equal(componenttest.Serialize(prometheus)))
				Expect(string(managedResourceSecret.Data["verticalpodautoscaler__some-namespace__prometheus-"+name+".yaml"])).To(Equal(componenttest.Serialize(vpa)))

				prometheusRule.Namespace = namespace
				metav1.SetMetaDataLabel(&prometheusRule.ObjectMeta, "prometheus", name)
				Expect(string(managedResourceSecret.Data["prometheusrule__some-namespace__"+name+"-rule.yaml"])).To(Equal(componenttest.Serialize(prometheusRule)))

				metav1.SetMetaDataLabel(&serviceMonitor.ObjectMeta, "prometheus", name)
				Expect(string(managedResourceSecret.Data["servicemonitor__default__"+name+"-monitor.yaml"])).To(Equal(componenttest.Serialize(serviceMonitor)))

				Expect(string(managedResourceSecret.Data["secret__some-namespace__prometheus-"+name+"-additional-scrape-configs.yaml"])).To(Equal(componenttest.Serialize(secretAdditionalScrapeConfigs)))
				Expect(string(managedResourceSecret.Data["configmap__some-namespace__configmap.yaml"])).To(Equal(componenttest.Serialize(additionalConfigMap)))
			})
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())
			Expect(fakeClient.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

			Expect(deployer.Destroy(ctx)).To(Succeed())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		Describe("#Wait", func() {
			It("should fail because reading the runtime ManagedResource fails", func() {
				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource is unhealthy", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: unhealthyManagedResourceStatus,
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should succeed because the ManagedResource is healthy and progressing", func() {
				Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesProgressing,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(deployer.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				Expect(fakeClient.Create(ctx, managedResource)).To(Succeed())

				Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it is already removed", func() {
				Expect(deployer.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

var (
	healthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
		ObservedGeneration: 1,
		Conditions: []gardencorev1beta1.Condition{
			{
				Type:   resourcesv1alpha1.ResourcesApplied,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   resourcesv1alpha1.ResourcesHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
		},
	}
	unhealthyManagedResourceStatus = resourcesv1alpha1.ManagedResourceStatus{
		ObservedGeneration: 1,
		Conditions: []gardencorev1beta1.Condition{
			{
				Type:   resourcesv1alpha1.ResourcesApplied,
				Status: gardencorev1beta1.ConditionFalse,
			},
			{
				Type:   resourcesv1alpha1.ResourcesHealthy,
				Status: gardencorev1beta1.ConditionFalse,
			},
		},
	}
)
