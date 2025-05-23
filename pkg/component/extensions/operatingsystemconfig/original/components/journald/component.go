// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package journald

import (
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
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
			Permissions: ptr.To[uint32](0644),
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
