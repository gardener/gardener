// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	_ "crypto/sha256"
	"fmt"

	"github.com/distribution/reference"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/logging/victorialogs"
)

// NewVictoriaLogs returns new VictoriaLogs deployer.
func NewVictoriaLogs(
	c client.Client,
	namespace string,
	clusterType component.ClusterType,
	replicas int32,
	priorityClassName string,
	storage *resource.Quantity,
	isGardenCluster bool,
	pvcAutoscaling victorialogs.PVCAutoscalingConfig,
) (
	component.DeployWaiter,
	error,
) {
	victoriaLogsImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameVictoriaLogs)
	if err != nil {
		return nil, err
	}

	// TODO(rrhubenov): Use the Image struct directly when issue #15259 is fixed.
	repository, tag, err := SplitImageRef(victoriaLogsImage.String())
	if err != nil {
		return nil, fmt.Errorf("failed parsing image %q from imagevector: %w", imagevector.ContainerImageNameVictoriaLogs, err)
	}

	deployer := victorialogs.New(c, namespace, victorialogs.Values{
		ImageRepository:   repository,
		ImageTag:          tag,
		Storage:           storage,
		IsGardenCluster:   isGardenCluster,
		ClusterType:       clusterType,
		Replicas:          replicas,
		PriorityClassName: priorityClassName,
		PVCAutoscaling:    pvcAutoscaling,
	})

	return deployer, nil
}

// SplitImageRef parses a fully-qualified image reference into its repository and tag components. When the reference
// contains both a tag and a digest, they are recombined into the '<tag>@sha256:...' form; a digest-only reference is
// returned as 'sha256:...'.
func SplitImageRef(image string) (repository string, tag string, err error) {
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return "", "", err
	}

	repository = ref.Name()

	if tagged, ok := ref.(reference.Tagged); ok {
		tag = tagged.Tag()
	}
	if digested, ok := ref.(reference.Digested); ok {
		digest := digested.Digest().String()
		if tag != "" {
			tag += "@" + digest
		} else {
			tag = digest
		}
	}

	return repository, tag, nil
}
