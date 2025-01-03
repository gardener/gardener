// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubelet

import (
	"fmt"
	"slices"

	"github.com/Masterminds/semver/v3"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// CLIFlags returns a list of kubelet CLI flags based on the provided parameters and for the provided Kubernetes version.
func CLIFlags(kubernetesVersion *semver.Version, nodeLabels map[string]string, criName extensionsv1alpha1.CRIName, cliFlags components.ConfigurableKubeletCLIFlags, preferIPv6 bool) []string {
	setCLIFlagsDefaults(&cliFlags)

	var flags []string

	flags = append(flags,
		"--bootstrap-kubeconfig="+PathKubeconfigBootstrap,
		"--config="+PathKubeletConfig,
		"--kubeconfig="+PathKubeconfigReal,
		fmt.Sprintf("--node-labels=%s=%s", v1beta1constants.LabelWorkerKubernetesVersion, kubernetesVersion.String()),
	)
	flags = append(flags, nodeLabelFlags(nodeLabels)...)

	if criName == extensionsv1alpha1.CRINameContainerD {
		flags = append(flags,
			"--container-runtime-endpoint="+containerd.PathSocketEndpoint,
			"--runtime-cgroups="+containerd.CgroupPath,
		)
	}

	flags = append(flags, "--v=2")
	// This is needed to prefer the ipv6 address over the ipv4 address in case the node has two addresses.
	// It's important for ipv6-only services with pods in the host network and for vpn, so that the ipv6 address of a node is used.
	if preferIPv6 {
		flags = append(flags, "--node-ip=\"::\"")
	}
	return flags
}

func setCLIFlagsDefaults(_ *components.ConfigurableKubeletCLIFlags) {
}

func nodeLabelFlags(nodeLabels map[string]string) []string {
	var flags []string

	for key := range nodeLabels {
		// Skip any node labels that kubelet is not allowed to add to its own Node object
		// (ref https://github.com/gardener/gardener/pull/7424).
		// The Worker extension (machine-controller-manager) is still responsible for adding/managing all worker pool labels
		// (even the ones we exclude here). I.e., they will get added to the Node object asynchronously.
		// We add all allowed worker pool labels (gardener-managed labels and allowed user-managed labels) via --node-labels
		// to prevent a race between our node controller and machine-controller-manager
		// (see https://github.com/gardener/gardener/issues/7117).
		// If users specify prohibited worker pool labels (e.g., node-role.kubernetes.io/default) that are relevant for
		// scheduling node-critical components (e.g., used in nodeSelectors of DaemonSets), they need to use different label
		// keys. However, excluding those prohibited labels here doesn't make things worse than they are today for them.
		// It only prevents them for making use of gardener's node readiness feature (see docs/usage/advanced/node-readiness.md).
		if !kubernetesutils.IsNodeLabelAllowedForKubelet(key) {
			continue
		}

		flags = append(flags, fmt.Sprintf("--node-labels=%s=%s", key, nodeLabels[key]))
	}

	// maps are unsorted in go, make sure to output node labels in the exact same order every time
	// this ensures deterministic behavior so that tests are stable and the OSC doesn't change on every reconciliation
	slices.Sort(flags)

	return flags
}
