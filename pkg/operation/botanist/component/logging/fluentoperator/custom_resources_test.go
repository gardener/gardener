// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package fluentoperator_test

import (
	"context"

	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/custom"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/filter"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/input"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/parser"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/logging/fluentoperator"
	componenttest "github.com/gardener/gardener/pkg/operation/botanist/component/test"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Fluent Operator Custom Resources", func() {
	var (
		ctx = context.TODO()

		fluentBitName         = "fluent-bit"
		namespace             = "some-namespace"
		image                 = "some-image:some-tag"
		priorityClassName     = "some-priority-class"
		resourceSelectorKey   = "fluentbit.gardener/type"
		resourceSelectorValue = "seed"
		values                = CustomResourcesValues{
			FluentBitImage:         image,
			FluentBitInitImage:     image,
			FluentBitPriorityClass: priorityClassName,
		}

		c         client.Client
		component component.DeployWaiter

		customResourcesManagedResourceName   = "fluent-operator-resources"
		customResourcesManagedResource       *resourcesv1alpha1.ManagedResource
		customResourcesManagedResourceSecret *corev1.Secret

		config *corev1.ConfigMap

		fluentBit              *fluentbitv1alpha2.FluentBit
		clusterFluentBitConfig *fluentbitv1alpha2.ClusterFluentBitConfig
		clusterInputs          []*fluentbitv1alpha2.ClusterInput
		clusterFilters         []*fluentbitv1alpha2.ClusterFilter
		clusterParsers         []*fluentbitv1alpha2.ClusterParser
		clusterOutputs         []*fluentbitv1alpha2.ClusterOutput

		configMapData map[string]string
		configMapName string
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		component = NewCustomResources(c, namespace, values, nil, nil, nil)

		configMapData = map[string]string{
			"modify_severity.lua": `
function cb_modify(tag, timestamp, record)
  local unified_severity = cb_modify_unify_severity(record)

  if not unified_severity then
    return 0, 0, 0
  end

  return 1, timestamp, record
end

function cb_modify_unify_severity(record)
  local modified = false
  local severity = record["severity"]
  if severity == nil or severity == "" then
	return modified
  end

  severity = trim(severity):upper()

  if severity == "I" or severity == "INF" or severity == "INFO" then
    record["severity"] = "INFO"
    modified = true
  elseif severity == "W" or severity == "WRN" or severity == "WARN" or severity == "WARNING" then
    record["severity"] = "WARN"
    modified = true
  elseif severity == "E" or severity == "ERR" or severity == "ERROR" or severity == "EROR" then
    record["severity"] = "ERR"
    modified = true
  elseif severity == "D" or severity == "DBG" or severity == "DEBUG" then
    record["severity"] = "DBG"
    modified = true
  elseif severity == "N" or severity == "NOTICE" then
    record["severity"] = "NOTICE"
    modified = true
  elseif severity == "F" or severity == "FATAL" then
    record["severity"] = "FATAL"
    modified = true
  end

  return modified
end

function trim(s)
  return (s:gsub("^%s*(.-)%s*$", "%1"))
end`,
			"add_tag_to_record.lua": `
function add_tag_to_record(tag, timestamp, record)
  record["tag"] = tag
  return 1, timestamp, record
end`,
		}

		configMapName = "fluent-bit-lua-config-" + utils.ComputeConfigMapChecksum(configMapData)[:8]

		config = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configMapName,
				Namespace: namespace,
				Labels: map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
				},
			},
			Data:      configMapData,
			Immutable: pointer.Bool(true),
		}

		fluentBit = &fluentbitv1alpha2.FluentBit{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fluentBitName,
				Namespace: namespace,
				Labels: map[string]string{
					v1beta1constants.LabelApp:                                            fluentBitName,
					v1beta1constants.LabelRole:                                           v1beta1constants.LabelLogging,
					v1beta1constants.GardenRole:                                          v1beta1constants.GardenRoleLogging,
					v1beta1constants.LabelNetworkPolicyToDNS:                             v1beta1constants.LabelNetworkPolicyAllowed,
					v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:                v1beta1constants.LabelNetworkPolicyAllowed,
					"networking.resources.gardener.cloud/to-logging-tcp-metrics":         v1beta1constants.LabelNetworkPolicyAllowed,
					"networking.resources.gardener.cloud/to-all-shoots-loki-tcp-metrics": v1beta1constants.LabelNetworkPolicyAllowed,
				},
			},
			Spec: fluentbitv1alpha2.FluentBitSpec{
				FluentBitConfigName: "fluent-bit-config",
				Image:               image,
				Command: []string{
					"/fluent-bit/bin/fluent-bit-watcher",
					"-e",
					"/fluent-bit/plugins/out_loki.so",
					"-c",
					"/fluent-bit/config/fluent-bit.conf",
				},
				PriorityClassName: priorityClassName,
				Ports: []corev1.ContainerPort{
					{
						Name:          "metrics-plugin",
						ContainerPort: 2021,
						Protocol:      "TCP",
					},
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("650Mi"),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("150m"),
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/api/v1/metrics/prometheus",
							Port: intstr.FromInt(2020),
						},
					},
					PeriodSeconds: 10,
				},
				LivenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/healthz",
							Port: intstr.FromInt(2021),
						},
					},
					PeriodSeconds:       300,
					InitialDelaySeconds: 90,
				},
				Tolerations: []corev1.Toleration{
					{
						Key:    "node-role.kubernetes.io/master",
						Effect: corev1.TaintEffectNoSchedule,
					},
					{
						Key:    "node-role.kubernetes.io/control-plane",
						Effect: corev1.TaintEffectNoSchedule,
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "runlogjournal",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/run/log/journal",
							},
						},
					},
					{
						Name: "optfluent",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{
								Path: "/opt/fluentbit",
							},
						},
					},
					{
						Name: "plugins",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
				VolumesMounts: []corev1.VolumeMount{
					{
						Name:      "runlogjournal",
						MountPath: "/run/log/journal",
					},
					{
						Name:      "optfluent",
						MountPath: "/opt/fluentbit",
					},
					{
						Name:      "plugins",
						MountPath: "/fluent-bit/plugins",
					},
				},
				InitContainers: []corev1.Container{
					{
						Name:  "install-plugin",
						Image: image,
						Command: []string{
							"cp",
							"/source/plugins/.",
							"/plugins",
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "plugins",
								MountPath: "/plugins",
							},
						},
					},
				},
				RBACRules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{"extensions.gardener.cloud"},
						Resources: []string{"clusters"},
						Verbs:     []string{"get", "list", "watch"},
					},
				},
				Service: fluentbitv1alpha2.FluentBitService{
					Name: fluentBitName,
					Annotations: map[string]string{
						resourcesv1alpha1.NetworkingFromPolicyPodLabelSelector: "all-seed-scrape-targets",
						resourcesv1alpha1.NetworkingFromPolicyAllowedPorts:     `[{"port":"2020","protocol":"TCP"},{"port":"2021","protocol":"TCP"}]`,
					},
					Labels: map[string]string{
						v1beta1constants.LabelApp:                                            fluentBitName,
						v1beta1constants.LabelRole:                                           v1beta1constants.LabelLogging,
						v1beta1constants.GardenRole:                                          v1beta1constants.GardenRoleLogging,
						v1beta1constants.LabelNetworkPolicyToDNS:                             v1beta1constants.LabelNetworkPolicyAllowed,
						v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer:                v1beta1constants.LabelNetworkPolicyAllowed,
						"networking.resources.gardener.cloud/to-logging-tcp-metrics":         v1beta1constants.LabelNetworkPolicyAllowed,
						"networking.resources.gardener.cloud/to-all-shoots-loki-tcp-metrics": v1beta1constants.LabelNetworkPolicyAllowed,
					},
				},
			},
		}

		clusterFluentBitConfig = &fluentbitv1alpha2.ClusterFluentBitConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: fluentBitName + "-config",
				Labels: map[string]string{
					"app.kubernetes.io/name": fluentBitName,
				},
			},
			Spec: fluentbitv1alpha2.FluentBitConfigSpec{
				Service: &fluentbitv1alpha2.Service{
					FlushSeconds: pointer.Int64(30),
					Daemon:       pointer.Bool(false),
					LogLevel:     "info",
					ParsersFile:  "parsers.conf",
					HttpServer:   pointer.Bool(true),
					HttpListen:   "0.0.0.0",
					HttpPort:     pointer.Int32(2020),
				},
				InputSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{resourceSelectorKey: resourceSelectorValue},
				},
				FilterSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{resourceSelectorKey: resourceSelectorValue},
				},
				ParserSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{resourceSelectorKey: resourceSelectorValue},
				},
				OutputSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{resourceSelectorKey: resourceSelectorValue},
				},
			},
		}

		clusterInputs = []*fluentbitv1alpha2.ClusterInput{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "tail-kubernetes",
					Labels: map[string]string{resourceSelectorKey: resourceSelectorValue},
				},
				Spec: fluentbitv1alpha2.InputSpec{
					Tail: &input.Tail{
						Tag:                    "kubernetes.*",
						Path:                   "/var/log/containers/*.log",
						ExcludePath:            "*_garden_fluent-bit-*.log,*_garden_loki-*.log",
						RefreshIntervalSeconds: pointer.Int64(10),
						MemBufLimit:            "30MB",
						SkipLongLines:          pointer.Bool(true),
						DB:                     "/opt/fluentbit/flb_kube.db",
						IgnoreOlder:            "30m",
					},
				},
			},
		}

		clusterFilters = []*fluentbitv1alpha2.ClusterFilter{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "01-docker",
					Labels: map[string]string{resourceSelectorKey: resourceSelectorValue},
				},
				Spec: fluentbitv1alpha2.FilterSpec{
					Match: "kubernetes.*",
					FilterItems: []fluentbitv1alpha2.FilterItem{
						{
							Parser: &filter.Parser{
								KeyName:     "log",
								Parser:      "docker-parser",
								ReserveData: pointer.Bool(true),
							},
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "02-containerd",
					Labels: map[string]string{resourceSelectorKey: resourceSelectorValue},
				},
				Spec: fluentbitv1alpha2.FilterSpec{
					Match: "kubernetes.*",
					FilterItems: []fluentbitv1alpha2.FilterItem{
						{
							Parser: &filter.Parser{
								KeyName:     "log",
								Parser:      "containerd-parser",
								ReserveData: pointer.Bool(true),
							},
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "03-add-tag-to-record",
					Labels: map[string]string{resourceSelectorKey: resourceSelectorValue},
				},
				Spec: fluentbitv1alpha2.FilterSpec{
					Match: "kubernetes.*",
					FilterItems: []fluentbitv1alpha2.FilterItem{
						{
							Lua: &filter.Lua{
								Script: corev1.ConfigMapKeySelector{
									Key: "add_tag_to_record.lua",
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapName,
									},
								},
								Call: "add_tag_to_record",
							},
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "zz-modify-severity",
					Labels: map[string]string{resourceSelectorKey: resourceSelectorValue},
				},
				Spec: fluentbitv1alpha2.FilterSpec{
					Match: "kubernetes.*",
					FilterItems: []fluentbitv1alpha2.FilterItem{
						{
							Lua: &filter.Lua{
								Script: corev1.ConfigMapKeySelector{
									Key: "modify_severity.lua",
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapName,
									},
								},
								Call: "cb_modify",
							},
						},
					},
				},
			},
		}

		clusterParsers = []*fluentbitv1alpha2.ClusterParser{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "docker-parser",
					Labels: map[string]string{resourceSelectorKey: resourceSelectorValue},
				},
				Spec: fluentbitv1alpha2.ParserSpec{
					JSON: &parser.JSON{
						TimeKey:    "time",
						TimeFormat: "%Y-%m-%dT%H:%M:%S.%L%z",
						TimeKeep:   pointer.Bool(true),
					},
					Decoders: []fluentbitv1alpha2.Decorder{
						{
							DecodeFieldAs: "json log",
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "containerd-parser",
					Labels: map[string]string{resourceSelectorKey: resourceSelectorValue},
				},
				Spec: fluentbitv1alpha2.ParserSpec{
					Regex: &parser.Regex{
						Regex:      "^(?<time>[^ ]+) (stdout|stderr) ([^ ]*) (?<log>.*)$",
						TimeKey:    "time",
						TimeFormat: "%Y-%m-%dT%H:%M:%S.%L%z",
						TimeKeep:   pointer.Bool(true),
					},
					Decoders: []fluentbitv1alpha2.Decorder{
						{
							DecodeFieldAs: "json log",
						},
					},
				},
			},
		}

		clusterOutputs = []*fluentbitv1alpha2.ClusterOutput{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "gardener-loki",
					Labels: map[string]string{resourceSelectorKey: resourceSelectorValue},
				},
				Spec: fluentbitv1alpha2.OutputSpec{
					CustomPlugin: &custom.CustomPlugin{
						Config: `Name gardenerloki
Match kubernetes.*
Url http://logging.garden.svc:3100/loki/api/v1/push
LogLevel info
BatchWait 60s
BatchSize 30720
Labels {origin="seed"}
LineFormat json
SortByTimestamp true
DropSingleKey false
AutoKubernetesLabels false
LabelSelector gardener.cloud/role:shoot
RemoveKeys kubernetes,stream,time,tag,gardenuser,job
LabelMapPath {"kubernetes": {"container_name":"container_name","container_id":"container_id","namespace_name":"namespace_name","pod_name":"pod_name"},"severity": "severity","job": "job"}
DynamicHostPath {"kubernetes": {"namespace_name": "namespace"}}
DynamicHostPrefix http://logging.
DynamicHostSuffix .svc:3100/loki/api/v1/push
DynamicHostRegex ^shoot-
DynamicTenant user gardenuser user
HostnameKeyValue nodename ${NODE_NAME}
MaxRetries 3
Timeout 10s
MinBackoff 30s
Buffer true
BufferType dque
QueueDir /fluent-bit/buffers/seed
QueueSegmentSize 300
QueueSync normal
QueueName gardener-kubernetes-operator
FallbackToTagWhenMetadataIsMissing true
TagKey tag
DropLogEntryWithoutK8sMetadata true
SendDeletedClustersLogsToDefaultClient true
CleanExpiredClientsPeriod 1h
ControllerSyncTimeout 120s
PreservedLabels origin,namespace_name,pod_name
NumberOfBatchIDs 5
TenantID operator`,
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "journald",
					Labels: map[string]string{resourceSelectorKey: resourceSelectorValue},
				},
				Spec: fluentbitv1alpha2.OutputSpec{
					CustomPlugin: &custom.CustomPlugin{
						Config: `Name gardenerloki
Match journald.*
Url http://logging.garden.svc:3100/loki/api/v1/push
LogLevel info
BatchWait 60s
BatchSize 30720
Labels {origin="seed-journald"}
LineFormat json
SortByTimestamp true
DropSingleKey false
RemoveKeys kubernetes,stream,hostname,unit
LabelMapPath {"hostname":"host_name","unit":"systemd_component"}
HostnameKeyValue nodename ${NODE_NAME}
MaxRetries 3
Timeout 10s
MinBackoff 30s
Buffer true
BufferType dque
QueueDir /fluent-bit/buffers
QueueSegmentSize 300
QueueSync normal
QueueName seed-journald
NumberOfBatchIDs 5`,
					},
				},
			},
		}
	})

	JustBeforeEach(func() {
		customResourcesManagedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      CustomResourcesManagedResourceName,
				Namespace: namespace,
			},
		}
		customResourcesManagedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + customResourcesManagedResource.Name,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		It("should successfully deploy all resources", func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResource), customResourcesManagedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, customResourcesManagedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResourceSecret), customResourcesManagedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, customResourcesManagedResourceSecret.Name)))

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResource), customResourcesManagedResource)).To(Succeed())
			Expect(customResourcesManagedResource).To(DeepEqual(&resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            CustomResourcesManagedResourceName,
					Namespace:       namespace,
					Labels:          map[string]string{v1beta1constants.GardenRole: "seed-system-component"},
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: pointer.String("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: customResourcesManagedResourceSecret.Name,
					}},
					KeepObjects: pointer.Bool(false),
				},
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResourceSecret), customResourcesManagedResourceSecret)).To(Succeed())
			Expect(customResourcesManagedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(customResourcesManagedResourceSecret.Data).To(HaveLen(12))

			Expect(string(customResourcesManagedResourceSecret.Data["configmap__"+namespace+"__"+configMapName+".yaml"])).To(Equal(componenttest.Serialize(config)))
			Expect(string(customResourcesManagedResourceSecret.Data["fluentbit__"+namespace+"__fluent-bit.yaml"])).To(Equal(componenttest.Serialize(fluentBit)))
			Expect(string(customResourcesManagedResourceSecret.Data["clusterfluentbitconfig____fluent-bit-config.yaml"])).To(Equal(componenttest.Serialize(clusterFluentBitConfig)))
			Expect(string(customResourcesManagedResourceSecret.Data["clusterinput____tail-kubernetes.yaml"])).To(Equal(componenttest.Serialize(clusterInputs[0])))
			Expect(string(customResourcesManagedResourceSecret.Data["clusterfilter____01-docker.yaml"])).To(Equal(componenttest.Serialize(clusterFilters[0])))
			Expect(string(customResourcesManagedResourceSecret.Data["clusterfilter____02-containerd.yaml"])).To(Equal(componenttest.Serialize(clusterFilters[1])))
			Expect(string(customResourcesManagedResourceSecret.Data["clusterfilter____03-add-tag-to-record.yaml"])).To(Equal(componenttest.Serialize(clusterFilters[2])))
			Expect(string(customResourcesManagedResourceSecret.Data["clusterfilter____zz-modify-severity.yaml"])).To(Equal(componenttest.Serialize(clusterFilters[3])))
			Expect(string(customResourcesManagedResourceSecret.Data["clusterparser____docker-parser.yaml"])).To(Equal(componenttest.Serialize(clusterParsers[0])))
			Expect(string(customResourcesManagedResourceSecret.Data["clusterparser____containerd-parser.yaml"])).To(Equal(componenttest.Serialize(clusterParsers[1])))
			Expect(string(customResourcesManagedResourceSecret.Data["clusteroutput____gardener-loki.yaml"])).To(Equal(componenttest.Serialize(clusterOutputs[0])))
			Expect(string(customResourcesManagedResourceSecret.Data["clusteroutput____journald.yaml"])).To(Equal(componenttest.Serialize(clusterOutputs[1])))
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(c.Create(ctx, customResourcesManagedResource)).To(Succeed())
			Expect(c.Create(ctx, customResourcesManagedResourceSecret)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResource), customResourcesManagedResource)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: resourcesv1alpha1.SchemeGroupVersion.Group, Resource: "managedresources"}, customResourcesManagedResource.Name)))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(customResourcesManagedResourceSecret), customResourcesManagedResourceSecret)).To(MatchError(apierrors.NewNotFound(schema.GroupResource{Group: corev1.SchemeGroupVersion.Group, Resource: "secrets"}, customResourcesManagedResourceSecret.Name)))
		})
	})

	Context("waiting functions", func() {
		var fakeOps *retryfake.Ops

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			DeferCleanup(test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			))
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResources fails", func() {
				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResources doesn't become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       customResourcesManagedResourceName,
						Namespace:  namespace,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
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
					},
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resources to become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       customResourcesManagedResourceName,
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
						},
					},
				})).To(Succeed())

				Expect(component.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resources deletion times out", func() {
				fakeOps.MaxAttempts = 2

				customResourcesManagedResource := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      customResourcesManagedResourceName,
						Namespace: namespace,
					},
				}
				Expect(c.Create(ctx, customResourcesManagedResource)).To(Succeed())

				Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(component.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})
