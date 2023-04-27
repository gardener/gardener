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

package customresources_test

import (
	fluentbitv1alpha2 "github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/custom"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/operation/botanist/component/logging/fluentoperator/customresources"
)

var _ = Describe("Logging", func() {
	Describe("#GetClusterOutputs", func() {
		var (
			labels = map[string]string{"some-key": "some-value"}
		)

		It("should return the expected ClusterOutput custom resources", func() {
			fluentBitClusterOutputs := GetClusterOutputs(labels)

			Expect(fluentBitClusterOutputs).To(Equal(
				[]*fluentbitv1alpha2.ClusterOutput{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "gardener-loki",
							Labels: labels,
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
							Labels: labels,
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
				}))
		})
	})
})
