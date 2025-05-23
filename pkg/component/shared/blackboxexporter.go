// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// NewBlackboxExporter creates a new blackbox-exporter deployer.
func NewBlackboxExporter(c client.Client, secretsManager secretsmanager.Interface, namespace string, values blackboxexporter.Values) (component.DeployWaiter, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameBlackboxExporter)
	if err != nil {
		return nil, err
	}
	values.Image = image.String()

	return blackboxexporter.New(c, secretsManager, namespace, values), nil
}
