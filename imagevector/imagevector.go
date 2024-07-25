// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../hack/generate-imagename-constants.sh
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
)

func init() {
	var err error

	containersImageVector, err = imagevector.Read([]byte(containersYAML))
	runtime.Must(err)
	containersImageVector, err = imagevector.WithEnvOverride(containersImageVector)
	runtime.Must(err)
}

// Containers is the image vector that contains all the needed container images.
func Containers() imagevector.ImageVector {
	return containersImageVector
}
