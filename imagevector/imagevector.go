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

	//go:embed charts.yaml
	chartsYAML        string
	chartsImageVector imagevector.ImageVector
)

func init() {
	var err error

	containersImageVector, err = imagevector.Read([]byte(containersYAML))
	runtime.Must(err)
	containersImageVector, err = imagevector.WithEnvOverride(containersImageVector, imagevector.OverrideEnv)
	runtime.Must(err)

	chartsImageVector, err = imagevector.Read([]byte(chartsYAML))
	runtime.Must(err)
	chartsImageVector, err = imagevector.WithEnvOverride(chartsImageVector, imagevector.OverrideChartsEnv)
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
