// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// NewEtcdDruid instantiates a new `etcd-druid` component.
func NewEtcdDruid(
	c client.Client,
	gardenNamespaceName string,
	runtimeVersion *semver.Version,
	imageVectorOverwrites map[string]string,
	etcdConfig *gardenletconfigv1alpha1.ETCDConfig,
	secretsManager secretsmanager.Interface,
	secretNameServerCA string,
	priorityClassName string,
) (
	component.DeployWaiter,
	error,
) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameEtcdDruid, imagevectorutils.RuntimeVersion(runtimeVersion.String()), imagevectorutils.TargetVersion(runtimeVersion.String()))
	if err != nil {
		return nil, err
	}

	var imageVectorOverwrite *string
	if val, ok := imageVectorOverwrites[etcd.Druid]; ok {
		imageVectorOverwrite = &val
	}

	return etcd.NewBootstrapper(
		c,
		gardenNamespaceName,
		runtimeVersion,
		etcdConfig,
		image.String(),
		imageVectorOverwrite,
		secretsManager,
		secretNameServerCA,
		priorityClassName,
	), nil
}
