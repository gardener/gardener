// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/containerd"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/Masterminds/semver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CLIFlags returns a list of kubelet CLI flags based on the provided parameters and for the provided Kubernetes version.
func CLIFlags(kubernetesVersion *semver.Version, criName extensionsv1alpha1.CRIName, image *imagevector.Image, cliFlags components.ConfigurableKubeletCLIFlags) []string {
	setCLIFlagsDefaults(&cliFlags)

	var flags []string

	flags = append(flags,
		"--bootstrap-kubeconfig="+PathKubeconfigBootstrap,
		"--config="+PathKubeletConfig,
		"--kubeconfig="+PathKubeconfigReal,
		fmt.Sprintf("--node-labels=%s=%s", v1beta1constants.LabelWorkerKubernetesVersion, kubernetesVersion.String()),
	)

	if version.ConstraintK8sLess119.Check(kubernetesVersion) {
		flags = append(flags, "--volume-plugin-dir="+pathVolumePluginDirectory)
	}

	if criName == extensionsv1alpha1.CRINameContainerD {
		flags = append(flags,
			"--container-runtime=remote",
			"--container-runtime-endpoint="+containerd.PathSocketEndpoint,
		)
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
