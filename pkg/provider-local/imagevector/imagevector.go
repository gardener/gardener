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

//go:generate ../../../hack/generate-imagename-constants.sh imagevector
package imagevector

import (
	_ "embed"
	"strings"

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

	imageVector, err = imagevector.Read(strings.NewReader(imagesYAML))
	runtime.Must(err)

	imageVector, err = imagevector.WithEnvOverride(imageVector)
	runtime.Must(err)
}

// ImageVector is the image vector that contains all the needed images.
func ImageVector() imagevector.ImageVector {
	return imageVector
}
