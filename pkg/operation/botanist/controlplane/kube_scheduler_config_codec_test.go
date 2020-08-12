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

package controlplane

import (
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	kubeschedulerv1alpha1 "k8s.io/kube-scheduler/config/v1alpha1"
	"k8s.io/utils/pointer"
)

var _ = Describe("KubeSchedulerConfigCodec", func() {
	var (
		c KubeSchedulerConfigCodec

		algorithmSourcePolicyFilePath       = "/foo/bar"
		bindTimeoutSeconds            int64 = 44
		schedulerName                       = "foo"
		healthzBindAddress                  = "1.2.3.4"
		metricsBindAddress                  = "5.6.7.8"
		leaderElectionNamespace             = "1"

		config = &kubeschedulerv1alpha1.KubeSchedulerConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: kubeschedulerv1alpha1.SchemeGroupVersion.String(),
				Kind:       "KubeSchedulerConfiguration",
			},
			AlgorithmSource: kubeschedulerv1alpha1.SchedulerAlgorithmSource{
				Policy: &kubeschedulerv1alpha1.SchedulerPolicySource{
					File: &kubeschedulerv1alpha1.SchedulerPolicyFileSource{
						Path: algorithmSourcePolicyFilePath,
					},
				},
			},
			BindTimeoutSeconds: &bindTimeoutSeconds,
			HealthzBindAddress: &healthzBindAddress,
			LeaderElection: kubeschedulerv1alpha1.KubeSchedulerLeaderElectionConfiguration{
				LeaderElectionConfiguration: componentbaseconfigv1alpha1.LeaderElectionConfiguration{
					ResourceNamespace: leaderElectionNamespace,
				},
			},
			MetricsBindAddress: &metricsBindAddress,
			SchedulerName:      pointer.StringPtr(schedulerName),
		}

		data = `algorithmSource:
  policy:
    file:
      path: ` + algorithmSourcePolicyFilePath + `
apiVersion: ` + kubeschedulerv1alpha1.SchemeGroupVersion.String() + `
bindTimeoutSeconds: ` + strconv.FormatInt(bindTimeoutSeconds, 10) + `
clientConnection:
  acceptContentTypes: ""
  burst: 0
  contentType: ""
  kubeconfig: ""
  qps: 0
healthzBindAddress: ` + healthzBindAddress + `
kind: KubeSchedulerConfiguration
leaderElection:
  leaderElect: null
  leaseDuration: 0s
  lockObjectName: ""
  lockObjectNamespace: ""
  renewDeadline: 0s
  resourceLock: ""
  resourceName: ""
  resourceNamespace: "` + leaderElectionNamespace + `"
  retryPeriod: 0s
metricsBindAddress: ` + metricsBindAddress + `
podInitialBackoffSeconds: null
podMaxBackoffSeconds: null
schedulerName: ` + schedulerName + `
`
	)

	BeforeEach(func() {
		c = NewKubeSchedulerConfigCodec()
	})

	Describe("#Encode", func() {
		It("should encode the given KubeSchedulerConfiguration into a string appropriately", func() {
			result, err := c.Encode(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(data))
		})
	})

	Describe("#Decode", func() {
		It("should decode a KubeSchedulerConfiguration from the given string appropriately", func() {
			result, err := c.Decode(data)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(config))
		})

		It("should return an error", func() {
			result, err := c.Decode("healthzBindAddress: 0")
			Expect(err).To(HaveOccurred())
			Expect(result).To(BeNil())
		})
	})
})
