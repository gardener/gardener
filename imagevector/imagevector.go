// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../hack/generate-imagename-constants.sh imagevector containers.yaml Container
//go:generate ../hack/resolve-etcd-version-from-etcd-druid.sh containers.yaml
//go:generate ../hack/generate-imagename-constants.sh imagevector charts.yaml Chart

package imagevector

import (
	_ "embed"

	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardener/pkg/utils/imagevector"
)

var (
	//go:embed containers.yaml
	containersYAML        string
	containersImageVector imagevector.ImageVector
	containersCABundle    *imagevector.CABundle

	//go:embed charts.yaml
	chartsYAML        string
	chartsImageVector imagevector.ImageVector
	chartsCABundle    *imagevector.CABundle
)

func init() {
	var err error

	containersImageVector, containersCABundle, err = imagevector.Read([]byte(containersYAML))
	runtime.Must(err)
	containersImageVector, containersCABundle, err = imagevector.WithEnvOverride(containersImageVector, containersCABundle, imagevector.OverrideEnv)
	runtime.Must(err)

	chartsImageVector, chartsCABundle, err = imagevector.Read([]byte(chartsYAML))
	runtime.Must(err)
	chartsImageVector, chartsCABundle, err = imagevector.WithEnvOverride(chartsImageVector, chartsCABundle, imagevector.OverrideChartsEnv)
	runtime.Must(err)
}

// Containers is the image vector that contains all the needed container images.
func Containers() imagevector.ImageVector {
	return containersImageVector
}

// Charts is the image vector that contains all the needed Helm chart images.
func Charts() imagevector.ImageVector {
	return chartsImageVector
}

// ContainersCABundle returns the CA bundle defined in the container image vector.
func ContainersCABundle() *imagevector.CABundle {
	return containersCABundle
}

// ChartsCABundle returns the CA bundle defined in the chart image vector.
func ChartsCABundle() *imagevector.CABundle {
	return chartsCABundle
}
