// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubeproxy

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeproxyv1alpha1 "k8s.io/kube-proxy/config/v1alpha1"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ConfigCodec", func() {
	var (
		kubeProxyConfig = &kubeproxyv1alpha1.KubeProxyConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "kubeproxy.config.k8s.io/v1alpha1",
				Kind:       "KubeProxyConfiguration",
			},
			ClusterCIDR: "1.2.3.4",
		}

		data = `apiVersion: kubeproxy.config.k8s.io/v1alpha1
bindAddress: ""
bindAddressHardFail: false
clientConnection:
  acceptContentTypes: ""
  burst: 0
  contentType: ""
  kubeconfig: ""
  qps: 0
clusterCIDR: 1.2.3.4
configSyncPeriod: 0s
conntrack:
  maxPerCore: null
  min: null
  tcpCloseWaitTimeout: null
  tcpEstablishedTimeout: null
detectLocal:
  bridgeInterface: ""
  interfaceNamePrefix: ""
detectLocalMode: ""
enableProfiling: false
healthzBindAddress: ""
hostnameOverride: ""
iptables:
  localhostNodePorts: null
  masqueradeAll: false
  masqueradeBit: null
  minSyncPeriod: 0s
  syncPeriod: 0s
ipvs:
  excludeCIDRs: null
  minSyncPeriod: 0s
  scheduler: ""
  strictARP: false
  syncPeriod: 0s
  tcpFinTimeout: 0s
  tcpTimeout: 0s
  udpTimeout: 0s
kind: KubeProxyConfiguration
metricsBindAddress: ""
mode: ""
nodePortAddresses: null
oomScoreAdj: null
portRange: ""
showHiddenMetricsForVersion: ""
winkernel:
  enableDSR: false
  forwardHealthCheckVip: false
  networkName: ""
  rootHnsEndpointName: ""
  sourceVip: ""
`
	)

	Describe("#Encode", func() {
		It("should encode the given KubeProxyConfiguration into bytes", func() {
			c := NewConfigCodec()

			result, err := c.Encode(kubeProxyConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(DeepEqual(data))
		})
	})

	Describe("#Decode", func() {
		It("should decode a KubeProxyConfiguration from the given bytes", func() {
			c := NewConfigCodec()

			config, err := c.Decode(data)
			Expect(err).NotTo(HaveOccurred())
			Expect(config).To(DeepEqual(kubeProxyConfig))
		})
	})
})
