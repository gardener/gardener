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
	"github.com/gardener/gardener/pkg/operation/botanist"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/pointer"
)

// GetOrDefaultDiffieHellmannKey creates an OpenVPN Diffie-Hellmann key
func (o *operation) GetOrDefaultDiffieHellmannKey(ctx context.Context) error {
	secret := &corev1.Secret{}
	if err := o.runtimeClient.Client().Get(ctx, kutil.Key(gardencorev1beta1constants.GardenNamespace, secretNameOpenVPNDiffieHellmann), secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		secret = nil
	}

	var (
		containsEncryptionConfig bool
		key                      []byte
	)

	if secret != nil {
		key, containsEncryptionConfig = secret.Data[secretDataKeyDiffieHellmann]
	}

	if secret == nil || !containsEncryptionConfig {
		o.log.Infof("Using default diffie-hellmann Key")
		key = []byte(botanist.DefaultDiffieHellmanKey)
	} else {
		o.log.Infof("Reusing OpenVPN Diffie-Hellmann key from the secret %s/%s in the runtime cluster", gardencorev1beta1constants.GardenNamespace, secretNameOpenVPNDiffieHellmann)
	}

	o.imports.OpenVPNDiffieHellmanKey = pointer.String(string(key))

	return nil
}
