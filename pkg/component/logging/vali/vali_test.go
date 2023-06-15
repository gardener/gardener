// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package vali_test

import (
	"context"

	"github.com/Masterminds/semver"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/logging/vali"
	"github.com/gardener/gardener/pkg/component/test"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

const (
	namespace             = "shoot--foo--bar"
	managedResourceName   = "vali"
	valiName              = "vali"
	valiConfigMapName     = "vali-config-e3b0c442"
	telegrafConfigMapName = "telegraf-config-e3b0c442"
	maintenanceBegin      = "210000-0000"
	maintenanceEnd        = "223000-0000"
	valiImage             = "vali:0.0.1"
	curatorImage          = "curator:0.0.1"
	alpineImage           = "alpine:0.0.1"
	initLargeDirImage     = "tune2fs:0.0.1"
	telegrafImage         = "telegraf-iptables:0.0.1"
	kubeRBACProxyImage    = "kube-rbac-proxy:0.0.1"
	priorityClassName     = "foo-bar"
	ingressClass          = "nginx"
	valiHost              = "vali.foo.bar"
)

var _ = Describe("Vali", func() {
	var (
		ctx = context.TODO()
		c   client.Client

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret

		fakeSecretManager secretsmanager.Interface
		k8sVersion        *semver.Version
		storage           = resource.MustParse("60Gi")
	)

	BeforeEach(func() {
		var err error
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeSecretManager = fakesecretsmanager.New(c, namespace)

		k8sVersion, err = semver.NewVersion("1.25.6")
		Expect(err).ToNot(HaveOccurred())

		By("Create secrets managed outside of this package for which secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
	})

	JustBeforeEach(func() {
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
	})

	Describe("#Deploy", func() {
		It("should successfully deploy all resources for shoot", func() {
			valiDeployer := New(
				c,
				namespace,
				fakeSecretManager,
				k8sVersion,
				Values{
					Replicas:         1,
					AuthEnabled:      true,
					Storage:          &storage,
					RBACProxyEnabled: true,
					HvpaEnabled:      true,
					MaintenanceTimeWindow: &hvpav1alpha1.MaintenanceTimeWindow{
						Begin: maintenanceBegin,
						End:   maintenanceEnd,
					},
					ValiImage:             valiImage,
					CuratorImage:          curatorImage,
					RenameLokiToValiImage: alpineImage,
					InitLargeDirImage:     initLargeDirImage,
					TelegrafImage:         telegrafImage,
					KubeRBACProxyImage:    kubeRBACProxyImage,
					PriorityClassName:     priorityClassName,
					ClusterType:           "shoot",
					IngressClass:          ingressClass,
					ValiHost:              valiHost,
				},
			)

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())

			Expect(valiDeployer.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(managedResource).To(DeepEqual(&resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceName,
					Namespace:       namespace,
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: pointer.String("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResourceSecret.Name,
					}},
					KeepObjects: pointer.Bool(false),
				},
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Data).To(HaveLen(6))

			Expect(string(managedResourceSecret.Data["configmap__shoot--foo--bar__telegraf-config-e3b0c442.yaml"])).To(Equal(test.Serialize(getTelegrafConfig())))
			Expect(string(managedResourceSecret.Data["configmap__shoot--foo--bar__vali-config-e3b0c442.yaml"])).To(Equal(test.Serialize(getValiConfig())))
			Expect(string(managedResourceSecret.Data["hvpa__shoot--foo--bar__vali.yaml"])).To(Equal(test.Serialize(getHVPA(true))))
			Expect(string(managedResourceSecret.Data["ingress__shoot--foo--bar__vali.yaml"])).To(Equal(test.Serialize(getKubeRBACProxyIngress())))
			Expect(string(managedResourceSecret.Data["service__shoot--foo--bar__logging.yaml"])).To(Equal(test.Serialize(getService(true, "shoot"))))
			Expect(string(managedResourceSecret.Data["statefulset__shoot--foo--bar__vali.yaml"])).To(Equal(test.Serialize(getStatefulset(true))))
		})

		It("should successfully deploy all resources for seed", func() {
			valiDeployer := New(
				c,
				namespace,
				fakeSecretManager,
				nil,
				Values{
					Replicas:    1,
					AuthEnabled: true,
					Storage:     &storage,
					HvpaEnabled: true,
					MaintenanceTimeWindow: &hvpav1alpha1.MaintenanceTimeWindow{
						Begin: maintenanceBegin,
						End:   maintenanceEnd,
					},
					ValiImage:             valiImage,
					CuratorImage:          curatorImage,
					RenameLokiToValiImage: alpineImage,
					InitLargeDirImage:     initLargeDirImage,
					PriorityClassName:     priorityClassName,
					ClusterType:           "seed",
				},
			)

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())

			Expect(valiDeployer.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(managedResource).To(DeepEqual(&resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceName,
					Namespace:       namespace,
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: pointer.String("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResourceSecret.Name,
					}},
					KeepObjects: pointer.Bool(false),
				},
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Data).To(HaveLen(4))

			Expect(string(managedResourceSecret.Data["configmap__shoot--foo--bar__vali-config-e3b0c442.yaml"])).To(Equal(test.Serialize(getValiConfig())))
			Expect(string(managedResourceSecret.Data["hvpa__shoot--foo--bar__vali.yaml"])).To(Equal(test.Serialize(getHVPA(false))))
			Expect(string(managedResourceSecret.Data["service__shoot--foo--bar__logging.yaml"])).To(Equal(test.Serialize(getService(false, "seed"))))
			Expect(string(managedResourceSecret.Data["statefulset__shoot--foo--bar__vali.yaml"])).To(Equal(test.Serialize(getStatefulset(false))))
		})

		It("should successfully deploy all resources for seed without HVPA", func() {
			valiDeployer := New(
				c,
				namespace,
				fakeSecretManager,
				nil,
				Values{
					Replicas:              1,
					AuthEnabled:           true,
					Storage:               &storage,
					ValiImage:             valiImage,
					CuratorImage:          curatorImage,
					RenameLokiToValiImage: alpineImage,
					InitLargeDirImage:     initLargeDirImage,
					PriorityClassName:     priorityClassName,
					ClusterType:           "seed",
				},
			)

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())

			Expect(valiDeployer.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(managedResource).To(DeepEqual(&resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceName,
					Namespace:       namespace,
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: pointer.String("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResourceSecret.Name,
					}},
					KeepObjects: pointer.Bool(false),
				},
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Data).To(HaveLen(3))

			Expect(string(managedResourceSecret.Data["configmap__shoot--foo--bar__vali-config-e3b0c442.yaml"])).To(Equal(test.Serialize(getValiConfig())))
			Expect(string(managedResourceSecret.Data["service__shoot--foo--bar__logging.yaml"])).To(Equal(test.Serialize(getService(false, "seed"))))
			Expect(string(managedResourceSecret.Data["statefulset__shoot--foo--bar__vali.yaml"])).To(Equal(test.Serialize(getStatefulset(false))))
		})
	})
})

func getService(isRBACProxyEnabled bool, clusterType string) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "logging",
			Namespace:   namespace,
			Labels:      getLabels(),
			Annotations: map[string]string{},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Port:       3100,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt(3100),
					Name:       "metrics",
				},
			},
			Selector: getLabels(),
		},
	}

	if isRBACProxyEnabled {
		svc.Spec.Ports = append(svc.Spec.Ports, []corev1.ServicePort{
			{
				Port:       8080,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(8080),
				Name:       "external",
			},
			{
				Port:       9273,
				Protocol:   corev1.ProtocolTCP,
				TargetPort: intstr.FromInt(9273),
				Name:       "telegraf",
			},
		}...)
	}

	switch clusterType {
	case "seed":
		if isRBACProxyEnabled {
			svc.Annotations["networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports"] = `[{"protocol":"TCP","port":3100},{"protocol":"TCP","port":9273}]`
		} else {
			svc.Annotations["networking.resources.gardener.cloud/from-all-seed-scrape-targets-allowed-ports"] = `[{"protocol":"TCP","port":3100}]`
		}
	case "shoot":
		if isRBACProxyEnabled {
			svc.Annotations["networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports"] = `[{"protocol":"TCP","port":3100},{"protocol":"TCP","port":9273}]`
		} else {
			svc.Annotations["networking.resources.gardener.cloud/from-all-scrape-targets-allowed-ports"] = `[{"protocol":"TCP","port":3100}]`
		}
		svc.Annotations["networking.resources.gardener.cloud/pod-label-selector-namespace-alias"] = "all-shoots"
		svc.Annotations["networking.resources.gardener.cloud/namespace-selectors"] = `[{"matchLabels":{"kubernetes.io/metadata.name":"garden"}}]`
	}

	return svc
}

func getValiConfig() *corev1.ConfigMap {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "vali-config",
			Namespace: namespace,
			Labels:    getLabels(),
		},
		BinaryData: map[string][]byte{
			"vali.yaml": []byte(`auth_enabled: true
ingester:
  chunk_target_size: 1536000
  chunk_idle_period: 3m
  chunk_block_size: 262144
  chunk_retain_period: 3m
  max_transfer_retries: 3
  lifecycler:
    ring:
      kvstore:
        store: inmemory
      replication_factor: 1
    final_sleep: 0s
    min_ready_duration: 1s
limits_config:
  enforce_metric_name: false
  reject_old_samples: true
  reject_old_samples_max_age: 168h
schema_config:
  configs:
  - from: 2018-04-15
    store: boltdb
    object_store: filesystem
    schema: v11
    index:
      prefix: index_
      period: 24h
server:
  http_listen_port: 3100
storage_config:
  boltdb:
    directory: /data/vali/index
  filesystem:
    directory: /data/vali/chunks
chunk_store_config:
  max_look_back_period: 360h
table_manager:
  retention_deletes_enabled: true
  retention_period: 360h
`),
			"curator.yaml": []byte(`LogLevel: info
DiskPath: /data/vali/chunks
TriggerInterval: 1h
InodeConfig:
  MinFreePercentages: 10
  TargetFreePercentages: 15
  PageSizeForDeletionPercentages: 1
StorageConfig:
  MinFreePercentages: 10
  TargetFreePercentages: 15
  PageSizeForDeletionPercentages: 1
`),
			"vali-init.sh": []byte(`#!/bin/bash
set -o errexit

function error() {
    exit_code=$?
    echo "${BASH_COMMAND} failed, exit code $exit_code"
}

trap error ERR

tune2fs -O large_dir $(mount | gawk '{if($3=="/data") {print $1}}')
`),
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	return configMap
}

func getTelegrafConfig() *corev1.ConfigMap {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "telegraf-config",
			Namespace: namespace,
			Labels:    getLabels(),
		},
		BinaryData: map[string][]byte{
			"telegraf.conf": []byte(`[[outputs.prometheus_client]]
## Address to listen on.
listen = ":9273"
metric_version = 2
# Gather packets and bytes throughput from iptables
[[inputs.iptables]]
## iptables require root access on most systems.
## Setting 'use_sudo' to true will make use of sudo to run iptables.
## Users must configure sudo to allow telegraf user to run iptables with no password.
## iptables can be restricted to only list command "iptables -nvL".
use_sudo = true
## defines the table to monitor:
table = "filter"
## defines the chains to monitor.
## NOTE: iptables rules without a comment will not be monitored.
## Read the plugin documentation for more information.
chains = [ "INPUT" ]
`),
			"start.sh": []byte(`#/bin/bash

trap 'kill %1; wait' SIGTERM
iptables -A INPUT -p tcp --dport 8080 -j ACCEPT -m comment --comment "valitail"
/usr/bin/telegraf --config /etc/telegraf/telegraf.conf &
wait
`),
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(configMap))

	return configMap
}

func getHVPA(isRBACProxyEnabled bool) *hvpav1alpha1.Hvpa {
	controlledValues := vpaautoscalingv1.ContainerControlledValuesRequestsOnly
	containerPolicyOff := vpaautoscalingv1.ContainerScalingModeOff

	obj := &hvpav1alpha1.Hvpa{
		ObjectMeta: metav1.ObjectMeta{
			Name:      valiName,
			Namespace: namespace,
			Labels:    getLabels(),
		},
		Spec: hvpav1alpha1.HvpaSpec{
			Replicas: pointer.Int32(1),
			MaintenanceTimeWindow: &hvpav1alpha1.MaintenanceTimeWindow{
				Begin: maintenanceBegin,
				End:   maintenanceEnd,
			},
			Hpa: hvpav1alpha1.HpaSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"role": valiName + "-hpa",
					},
				},
				Deploy: false,
				Template: hvpav1alpha1.HpaTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"role": valiName + "-hpa",
						},
					},
					Spec: hvpav1alpha1.HpaTemplateSpec{
						MinReplicas: pointer.Int32(1),
						MaxReplicas: 1,
						Metrics: []autoscalingv2beta1.MetricSpec{
							{
								Type: "Resource",
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     "cpu",
									TargetAverageUtilization: pointer.Int32(80),
								},
							},
							{
								Type: "Resource",
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     "memory",
									TargetAverageUtilization: pointer.Int32(80),
								},
							},
						},
					},
				},
			},
			Vpa: hvpav1alpha1.VpaSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"role": valiName + "vpa",
					},
				},
				Deploy: true,
				ScaleUp: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: pointer.String("Auto"),
					},
					StabilizationDuration: pointer.String("5m"),
					MinChange: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("100m"),
							Percentage: pointer.Int32(80),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("300M"),
							Percentage: pointer.Int32(80),
						},
					},
				},
				ScaleDown: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: pointer.String("MaintenanceWindow"),
					},
					StabilizationDuration: pointer.String("168h"),
					MinChange: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("200m"),
							Percentage: pointer.Int32(80),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("500M"),
							Percentage: pointer.Int32(80),
						},
					},
				},
				LimitsRequestsGapScaleParams: hvpav1alpha1.ScaleParams{
					CPU: hvpav1alpha1.ChangeParams{
						Value:      pointer.String("300m"),
						Percentage: pointer.Int32(40),
					},
					Memory: hvpav1alpha1.ChangeParams{
						Value:      pointer.String("1G"),
						Percentage: pointer.Int32(40),
					},
				},
				Template: hvpav1alpha1.VpaTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"role": valiName + "vpa",
						},
					},
					Spec: hvpav1alpha1.VpaTemplateSpec{
						ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
							ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
								{
									ContainerName: valiName,
									MinAllowed: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("300M"),
									},
									MaxAllowed: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("800m"),
										corev1.ResourceMemory: resource.MustParse("3Gi"),
									},
									ControlledValues: &controlledValues,
								},
								{
									ContainerName:    "curator",
									Mode:             &containerPolicyOff,
									ControlledValues: &controlledValues,
								},
								{
									ContainerName:    "init-large-dir",
									Mode:             &containerPolicyOff,
									ControlledValues: &controlledValues,
								},
							},
						},
					},
				},
			},
			WeightBasedScalingIntervals: []hvpav1alpha1.WeightBasedScalingInterval{
				{
					VpaWeight:         hvpav1alpha1.VpaOnly,
					StartReplicaCount: 1,
					LastReplicaCount:  1,
				},
			},
			TargetRef: &autoscalingv2beta1.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
				Name:       valiName,
			},
		},
	}

	if isRBACProxyEnabled {
		obj.Spec.Vpa.Template.Spec.ResourcePolicy.ContainerPolicies = append(obj.Spec.Vpa.Template.Spec.ResourcePolicy.ContainerPolicies,
			[]vpaautoscalingv1.ContainerResourcePolicy{
				{
					ContainerName:    "kube-rbac-proxy",
					Mode:             &containerPolicyOff,
					ControlledValues: &controlledValues,
				},
				{
					ContainerName:    "telegraf",
					Mode:             &containerPolicyOff,
					ControlledValues: &controlledValues,
				},
			}...)
	}
	return obj
}

func getKubeRBACProxyIngress() *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      valiName,
			Namespace: namespace,
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/configuration-snippet": "proxy_set_header X-Scope-OrgID operator;",
			},
			Labels: getLabels(),
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: pointer.String(ingressClass),
			TLS: []networkingv1.IngressTLS{
				{
					SecretName: "vali-tls",
					Hosts:      []string{valiHost},
				},
			},
			Rules: []networkingv1.IngressRule{
				{
					Host: valiHost,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "vali",
											Port: networkingv1.ServiceBackendPort{
												Number: 8080,
											},
										},
									},
									Path:     "/vali/api/v1/push",
									PathType: &pathType,
								},
							},
						},
					},
				},
			},
		},
	}
}

func getStatefulset(isRBACProxyEnabled bool) *appsv1.StatefulSet {
	fsGroupChangeOnRootMismatch := corev1.FSGroupChangeOnRootMismatch
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      valiName,
			Namespace: namespace,
			Labels:    getLabels(),
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointer.Int32(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: getLabels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: getLabels(),
				},
				Spec: corev1.PodSpec{
					PriorityClassName:            priorityClassName,
					AutomountServiceAccountToken: pointer.Bool(false),
					SecurityContext: &corev1.PodSecurityContext{
						FSGroup:             pointer.Int64(10001),
						FSGroupChangePolicy: &fsGroupChangeOnRootMismatch,
					},
					InitContainers: []corev1.Container{
						{
							Name:  "init-large-dir",
							Image: initLargeDirImage,
							Command: []string{
								"bash",
								"-c",
								"/vali-init.sh || true",
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged:   pointer.Bool(true),
								RunAsUser:    pointer.Int64(0),
								RunAsNonRoot: pointer.Bool(false),
								RunAsGroup:   pointer.Int64(0),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/data",
									Name:      "vali",
								},
								{
									MountPath: "/vali-init.sh",
									SubPath:   "vali-init.sh",
									Name:      "config",
								},
							},
						},
						{
							Name:  "rename-loki-to-vali",
							Image: alpineImage,
							Command: []string{
								"sh",
								"-c",
								`set -x
								# TODO (istvanballok): remove in release v1.77
								if [[ -d /data/loki ]]; then
								  echo "Renaming loki folder to vali"
								  time mv /data/loki /data/vali
								else
								  echo "No loki folder found"
								fi`,
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/data",
									Name:      "vali",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "vali",
							Image: valiImage,
							Args: []string{
								"-config.file=/etc/vali/vali.yaml",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/etc/vali/vali.yaml",
									SubPath:   "vali.yaml",
								},
								{
									Name:      "vali",
									MountPath: "/data",
								},
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "metrics",
									ContainerPort: 3100,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromString("metrics"),
									},
								},
								InitialDelaySeconds: 120,
								FailureThreshold:    5,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/ready",
										Port: intstr.FromString("metrics"),
									},
								},
								FailureThreshold: 7,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("300Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("3Gi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:              pointer.Int64(10001),
								RunAsGroup:             pointer.Int64(10001),
								RunAsNonRoot:           pointer.Bool(true),
								ReadOnlyRootFilesystem: pointer.Bool(true),
							},
						},
						{
							Name:  "curator",
							Image: curatorImage,
							Args: []string{
								"-config=/etc/vali/curator.yaml",
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "curatormetrics",
									ContainerPort: 2718,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config",
									MountPath: "/etc/vali/curator.yaml",
									SubPath:   "curator.yaml",
								},
								{
									Name:      "vali",
									MountPath: "/data",
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("10m"),
									corev1.ResourceMemory: resource.MustParse("12Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("700Mi"),
								},
							},
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:              pointer.Int64(10001),
								RunAsGroup:             pointer.Int64(10001),
								RunAsNonRoot:           pointer.Bool(true),
								ReadOnlyRootFilesystem: pointer.Bool(true),
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: valiConfigMapName,
									},
									DefaultMode: pointer.Int32(0520),
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "vali",
					},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{
							"ReadWriteOnce",
						},
						Resources: corev1.ResourceRequirements{
							Requests: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceStorage: resource.MustParse("60Gi"),
							},
						},
					},
				},
			},
		},
	}

	if isRBACProxyEnabled {
		sts.Spec.Template.ObjectMeta.Labels["networking.gardener.cloud/to-dns"] = "allowed"
		sts.Spec.Template.ObjectMeta.Labels["networking.resources.gardener.cloud/to-kube-apiserver-tcp-443"] = "allowed"

		sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, []corev1.Container{
			{
				Name:  "kube-rbac-proxy",
				Image: kubeRBACProxyImage,
				Args: []string{
					"--insecure-listen-address=0.0.0.0:8080",
					"--upstream=http://127.0.0.1:3100/",
					"--kubeconfig=/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig/kubeconfig",
					"--logtostderr=true",
					"--v=6",
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("150Mi"),
					},
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "kube-rbac-proxy",
						ContainerPort: 8080,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "kubeconfig",
						MountPath: "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig",
						ReadOnly:  true,
					},
				},
				SecurityContext: &corev1.SecurityContext{
					RunAsUser:              pointer.Int64(65532),
					RunAsGroup:             pointer.Int64(65534),
					RunAsNonRoot:           pointer.Bool(true),
					ReadOnlyRootFilesystem: pointer.Bool(true),
				},
			},
			{
				Name:  "telegraf",
				Image: telegrafImage,
				Command: []string{
					"/bin/bash",
					"-c",
					`            trap 'kill %1; wait' SIGTERM
					bash /etc/telegraf/start.sh &
					wait`,
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("5m"),
						corev1.ResourceMemory: resource.MustParse("35Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("350Mi"),
					},
				},
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Add: []corev1.Capability{
							"NET_ADMIN",
						},
					},
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "telegraf",
						ContainerPort: 9273,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "telegraf-config-volume",
						MountPath: "/etc/telegraf/telegraf.conf",
						SubPath:   "telegraf.conf",
						ReadOnly:  true,
					},
					{
						Name:      "telegraf-config-volume",
						MountPath: "/etc/telegraf/start.sh",
						SubPath:   "start.sh",
						ReadOnly:  true,
					},
				},
			},
		}...)

		sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, []corev1.Volume{
			{
				Name: "kubeconfig",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						DefaultMode: pointer.Int32(420),
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									Items: []corev1.KeyToPath{
										{
											Key:  "kubeconfig",
											Path: "kubeconfig",
										},
									},
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "generic-token-kubeconfig",
									},
									Optional: pointer.Bool(false),
								},
							},
							{
								Secret: &corev1.SecretProjection{
									Items: []corev1.KeyToPath{
										{
											Key:  "token",
											Path: "token",
										},
									},
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "shoot-access-" + "kube-rbac-proxy",
									},
									Optional: pointer.Bool(false),
								},
							},
						},
					},
				},
			},
			{
				Name: "telegraf-config-volume",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: telegrafConfigMapName,
						},
					},
				},
			},
		}...)
	}

	utilruntime.Must(references.InjectAnnotations(sts))

	return sts
}

func getLabels() map[string]string {
	return map[string]string{
		"gardener.cloud/role": "logging",
		"role":                "logging",
		"app":                 "vali",
	}
}
