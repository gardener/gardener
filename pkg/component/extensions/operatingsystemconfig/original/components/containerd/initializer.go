// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	_ "embed"

	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/utils"
)

var (
	//go:embed templates/scripts/init.sh
	contentInitializer []byte
)

type initializer struct{}

// NewInitializer returns a new containerd initializer component.
// Deprecated: The containerd initializer is deprecated and will be removed in a future version. Please don't change or add content to the init script.
// TODO(timuthy): Remove Initializer after Gardener v1.114 was released.
func NewInitializer() *initializer {
	return &initializer{}
}

func (initializer) Name() string {
	return "containerd-initializer"
}

func (initializer) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	const (
		pathScript          = v1beta1constants.OperatingSystemConfigFilePathBinaries + "/init-containerd"
		unitNameInitializer = "containerd-initializer.service"
	)

	return []extensionsv1alpha1.Unit{
			{
				Name:    unitNameInitializer,
				Command: ptr.To(extensionsv1alpha1.CommandStart),
				Enable:  ptr.To(true),
				Content: ptr.To(`[Unit]
Description=Containerd initializer
[Install]
WantedBy=multi-user.target
[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=` + pathScript),
			},
		},
		[]extensionsv1alpha1.File{
			{
				Path:        pathScript,
				Permissions: ptr.To[int32](744),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64(contentInitializer),
					},
				},
			},
		},
		nil
}
