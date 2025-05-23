// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"github.com/go-logr/logr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/alertmanager"
)

// NewAlertmanager creates a new alertmanager deployer.
func NewAlertmanager(log logr.Logger, c client.Client, namespace string, values alertmanager.Values) (alertmanager.Interface, error) {
	imageAlertmanager, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameAlertmanager)
	if err != nil {
		return nil, err
	}

	values.Image = imageAlertmanager.String()
	values.Version = ptr.Deref(imageAlertmanager.Version, "v0.0.0")

	return alertmanager.New(log, c, namespace, values), nil
}
