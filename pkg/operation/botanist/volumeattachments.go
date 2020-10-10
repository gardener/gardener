// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/sirupsen/logrus"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeleteVolumeAttachments deletes all VolumeAttachments.
func DeleteVolumeAttachments(ctx context.Context, c client.Client) error {
	return c.DeleteAllOf(
		ctx,
		&storagev1beta1.VolumeAttachment{},
	)
}

// WaitUntilVolumeAttachmentsDeleted waits until no VolumeAttachments exist anymore.
func WaitUntilVolumeAttachmentsDeleted(ctx context.Context, c client.Client, log *logrus.Entry) error {
	return retry.Until(ctx, 5*time.Second, func(ctx context.Context) (done bool, err error) {
		vaList := &storagev1beta1.VolumeAttachmentList{}
		if err := c.List(ctx, vaList); err != nil {
			return retry.SevereError(err)
		}

		if len(vaList.Items) == 0 {
			return retry.Ok()
		}

		log.Infof("Waiting until all VolumeAttachments have been deleted in the shoot cluster...")
		return retry.MinorError(fmt.Errorf("not all VolumeAttachments have been deleted in the shoot cluster"))
	})
}
