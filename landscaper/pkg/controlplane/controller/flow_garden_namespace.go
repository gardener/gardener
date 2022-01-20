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

package controller

import (
	"context"

	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PrepareGardenNamespace prepares the Garden namespace in the Garden cluster
func (o *operation) PrepareGardenNamespace(ctx context.Context) error {
	client := o.getGardenClient().Client()

	gardenNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: gardencorev1beta1constants.GardenNamespace,
		},
	}

	// create + label garden namespace
	if _, err := controllerutils.CreateOrGetAndMergePatch(ctx, client, gardenNamespace, func() error {
		metav1.SetMetaDataLabel(&gardenNamespace.ObjectMeta, gardencorev1beta1constants.GardenRole, gardencorev1beta1constants.GardenRoleProject)
		metav1.SetMetaDataLabel(&gardenNamespace.ObjectMeta, gardencorev1beta1constants.ProjectName, gardencorev1beta1constants.GardenRoleProject)
		metav1.SetMetaDataLabel(&gardenNamespace.ObjectMeta, gardencorev1beta1constants.LabelApp, "gardener")
		return nil
	}); err != nil {
		return err
	}

	return nil
}
