// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../../hack/generate-imagename-constants.sh imagevector images.yaml
package imagevector

import (
	_ "embed"

	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/gardener/gardener/pkg/utils/imagevector"
)

var (
	//go:embed images.yaml
	imagesYAML  string
	imageVector imagevector.ImageVector
)

func init() {
	var err error

	imageVector, err = imagevector.Read([]byte(imagesYAML))
	runtime.Must(err)

	imageVector, err = imagevector.WithEnvOverride(imageVector)
	runtime.Must(err)
}

// ImageVector is the image vector that contains all the needed images.
func ImageVector() imagevector.ImageVector {
	return imageVector
}
