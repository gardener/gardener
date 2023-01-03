// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	v1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	utilerrors "github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/hashicorp/go-multierror"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DeleteBastions deletes all bastions from the Shoot namespace in the Seed.
func (b *Botanist) DeleteBastions(ctx context.Context) error {
	return extensions.DeleteExtensionObjects(
		ctx,
		b.SeedClientSet.Client(),
		&v1alpha1.BastionList{},
		b.Shoot.SeedNamespace,
		func(obj v1alpha1.Object) bool {
			return true
		},
	)
}

// WaitUntilBastionsDeleted waits until all bastions for the shoot are deleted.
func (b *Botanist) WaitUntilBastionsDeleted(ctx context.Context) error {
	return retry.Until(ctx, time.Second*5, func(ctx context.Context) (done bool, err error) {
		bastionList := &v1alpha1.BastionList{}
		if err := b.SeedClientSet.Client().List(ctx, bastionList, client.InNamespace(b.Shoot.SeedNamespace)); err != nil {
			return retry.SevereError(err)
		}

		allErrs := &multierror.Error{
			ErrorFormat: utilerrors.NewErrorFormatFuncWithPrefix("error while waiting for all bastions to be deleted: "),
		}
		for _, bastion := range bastionList.Items {
			allErrs = multierror.Append(allErrs, fmt.Errorf("bastion %s/%s still exists", bastion.ObjectMeta.Namespace, bastion.ObjectMeta.Name))
		}

		if err := allErrs.ErrorOrNil(); err != nil {
			return retry.MinorError(err)
		}

		return retry.Ok()
	})
}
