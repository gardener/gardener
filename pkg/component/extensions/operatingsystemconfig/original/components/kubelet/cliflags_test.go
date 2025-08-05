// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubelet_test

import (
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/kubelet"
)

var _ = Describe("CLIFlags", func() {
	DescribeTable("#CLIFlags",
		func(criName extensionsv1alpha1.CRIName, cliFlags components.ConfigurableKubeletCLIFlags, preferIPv6 bool, matcher gomegatypes.GomegaMatcher) {
			nodeLabels := map[string]string{
				"test":  "foo",
				"test2": "bar",
				// assert that we only pass allowed node labels to --node-labels
				"kubernetes.io/arch":                            "amd64",  // allowed
				"k8s.io/foo":                                    "bar",    // not allowed
				"bar.k8s.io/foo":                                "bar",    // not allowed
				"node-role.kubernetes.io/default":               "worker", // not allowed
				"worker.gardener.cloud/pool":                    "worker", // allowed
				"containerruntime.worker.gardener.cloud/gvisor": "true",   // allowed
			}
			Expect(kubelet.CLIFlags(semver.MustParse("1.32.0"), nodeLabels, criName, cliFlags, preferIPv6)).To(matcher)
		},

		Entry(
			"w/ containerd",
			extensionsv1alpha1.CRINameContainerD,
			components.ConfigurableKubeletCLIFlags{},
			false,
			ConsistOf(
				"--bootstrap-kubeconfig=/var/lib/kubelet/kubeconfig-bootstrap",
				"--config=/var/lib/kubelet/config/kubelet",
				"--kubeconfig=/var/lib/kubelet/kubeconfig-real",
				"--node-labels=worker.gardener.cloud/kubernetes-version=1.32.0",
				"--node-labels=containerruntime.worker.gardener.cloud/gvisor=true",
				"--node-labels=kubernetes.io/arch=amd64",
				"--node-labels=test=foo",
				"--node-labels=test2=bar",
				"--node-labels=worker.gardener.cloud/pool=worker",
				"--container-runtime-endpoint=unix:///run/containerd/containerd.sock",
				"--runtime-cgroups=/system.slice/containerd.service",
				"--v=2",
			),
		),
		Entry(
			"w/ containerd w/ preferIPv6",
			extensionsv1alpha1.CRINameContainerD,
			components.ConfigurableKubeletCLIFlags{},
			true,
			ConsistOf(
				"--bootstrap-kubeconfig=/var/lib/kubelet/kubeconfig-bootstrap",
				"--config=/var/lib/kubelet/config/kubelet",
				"--kubeconfig=/var/lib/kubelet/kubeconfig-real",
				"--node-labels=worker.gardener.cloud/kubernetes-version=1.32.0",
				"--node-labels=containerruntime.worker.gardener.cloud/gvisor=true",
				"--node-labels=kubernetes.io/arch=amd64",
				"--node-labels=test=foo",
				"--node-labels=test2=bar",
				"--node-labels=worker.gardener.cloud/pool=worker",
				"--container-runtime-endpoint=unix:///run/containerd/containerd.sock",
				"--runtime-cgroups=/system.slice/containerd.service",
				"--v=2",
				"--node-ip=\"::\"",
			),
		),
	)
})
