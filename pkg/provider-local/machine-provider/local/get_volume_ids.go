// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"context"

	"github.com/gardener/machine-controller-manager/pkg/util/provider/driver"
)

func (d *localDriver) GetVolumeIDs(_ context.Context, _ *driver.GetVolumeIDsRequest) (*driver.GetVolumeIDsResponse, error) {
	// TODO: In the future, this could return the volumes for a local provisioner.
	return &driver.GetVolumeIDsResponse{}, nil
}
