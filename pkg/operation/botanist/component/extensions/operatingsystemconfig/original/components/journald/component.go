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

package journald

import (
	"k8s.io/utils/pointer"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
)

type component struct{}

// New returns a new journald component.
func New() *component {
	return &component{}
}

func (component) Name() string {
	return "journald"
}

func (component) Config(_ components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	return nil, []extensionsv1alpha1.File{
		{
			Path:        "/etc/systemd/journald.conf",
			Permissions: pointer.Int32(0644),
			Content: extensionsv1alpha1.FileContent{
				Inline: &extensionsv1alpha1.FileContentInline{
					Data: `[Journal]
MaxFileSec=24h
MaxRetentionSec=14day
`,
				},
			},
		},
	}, nil
}
