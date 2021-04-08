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

package managedseed

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func (c *Controller) filterSeed(obj, _, controller client.Object, deleted bool) bool {
	seed, ok := obj.(*gardencorev1beta1.Seed)
	if !ok {
		return false
	}
	_, ok = controller.(*seedmanagementv1alpha1.ManagedSeed)
	if !ok {
		return false
	}

	if checksum := seed.Annotations[v1beta1constants.AnnotationModelChecksum]; checksum != utils.ComputeChecksum(gardencorev1beta1helper.GetSeedModel(seed)) {
		c.logger.Debugf("Model checksum of seed %s doesn't match model", kutil.ObjectName(seed))
		return true
	}
	if deleted {
		c.logger.Debugf("Seed %s no longer exists", kutil.ObjectName(seed))
		return true
	}
	return false
}

func (c *Controller) filterSecret(obj, _, controller client.Object, deleted bool) bool {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return false
	}
	_, ok = controller.(*seedmanagementv1alpha1.ManagedSeed)
	if !ok {
		return false
	}

	if checksum := secret.Annotations[v1beta1constants.AnnotationModelChecksum]; checksum != utils.ComputeChecksum(kutil.GetSecretModel(secret)) {
		c.logger.Debugf("Model checksum of secret %s doesn't match model", kutil.ObjectName(secret))
		return true
	}
	if deleted {
		c.logger.Debugf("Secret %s no longer exists", kutil.ObjectName(secret))
		return true
	}
	return false
}
