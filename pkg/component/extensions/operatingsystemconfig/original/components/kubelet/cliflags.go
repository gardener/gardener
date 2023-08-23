// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubelet

import (
	"fmt"
	"slices"
	"time"

	"github.com/Masterminds/semver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// CLIFlags returns a list of kubelet CLI flags based on the provided parameters and for the provided Kubernetes version.
func CLIFlags(kubernetesVersion *semver.Version, nodeLabels map[string]string, criName extensionsv1alpha1.CRIName, image *imagevector.Image, cliFlags components.ConfigurableKubeletCLIFlags) []string {
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
		if versionutils.ConstraintK8sLess127.Check(kubernetesVersion) {
			flags = append(flags, "--container-runtime=remote")
		}
	} else if criName == extensionsv1alpha1.CRINameDocker {
		flags = append(flags,
			"--network-plugin=cni",
			"--cni-bin-dir=/opt/cni/bin/",
			"--cni-conf-dir=/etc/cni/net.d/",
			fmt.Sprintf("--image-pull-progress-deadline=%s", cliFlags.ImagePullProgressDeadline.Duration.String()))
		if image != nil {
			flags = append(flags, "--pod-infra-container-image="+image.String())
		}
	}

	flags = append(flags, "--v=2")

	return flags
}

func setCLIFlagsDefaults(f *components.ConfigurableKubeletCLIFlags) {
	if f.ImagePullProgressDeadline == nil {
		f.ImagePullProgressDeadline = &metav1.Duration{Duration: time.Minute}
	}
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
		// It only prevents them for making use of gardener's node readiness feature (see docs/usage/node-readiness.md).
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
