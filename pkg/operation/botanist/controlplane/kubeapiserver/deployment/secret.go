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

package deployment

import (
	"bytes"
	"context"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (k *kubeAPIServer) deploySecretOIDCBundle(ctx context.Context) (*string, error) {
	var (
		secretOIDCBundle = k.emptySecret(secretNameOIDC)
		buf              = new(bytes.Buffer)
		sha256Hex        string
	)

	// create a copy to create SHA sum - do not create SHA based on current resource that includes the resource version
	secretOIDCSHA := *secretOIDCBundle
	secretOIDCSHA.Data = map[string][]byte{
		fileNameSecretOIDCCert: []byte(utils.EncodeBase64([]byte(*k.config.OIDCConfig.CABundle))),
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, k.seedClient.Client(), secretOIDCBundle, func() error {
		secretOIDCBundle.Data = secretOIDCSHA.Data
		return nil
	}); err != nil {
		return nil, err
	}

	if err := encoderCoreV1.Encode(&secretOIDCSHA, buf); err != nil {
		return nil, err
	}

	sha256Hex = utils.ComputeSHA256Hex(buf.Bytes())
	return &sha256Hex, nil
}

func (k *kubeAPIServer) deploySecretServiceAccountSigningKey(ctx context.Context) (*string, error) {
	var (
		secretServiceAccountSigningKey = k.emptySecret(secretNameServiceAccountSigningKey)
		buf                            = new(bytes.Buffer)
		sha256Hex                      string
	)

	signingKey, err := common.GetServiceAccountSigningKeySecret(ctx, k.gardenClient, k.gardenNamespace, k.config.ServiceAccountConfig.SigningKeySecret.Name)
	if err != nil {
		return nil, err
	}

	// create a copy to create SHA sum - do not create SHA based on current resource that includes the resource version
	secretServiceAccountSHA := *secretServiceAccountSigningKey
	secretServiceAccountSHA.Data = map[string][]byte{
		fileNameServiceAccountSigning: []byte(utils.EncodeBase64([]byte(signingKey))),
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, k.seedClient.Client(), secretServiceAccountSigningKey, func() error {
		secretServiceAccountSigningKey.Data = secretServiceAccountSHA.Data
		return nil
	}); err != nil {
		return nil, err
	}

	if err := encoderCoreV1.Encode(&secretServiceAccountSHA, buf); err != nil {
		return nil, err
	}

	sha256Hex = utils.ComputeSHA256Hex(buf.Bytes())
	return &sha256Hex, nil
}

func (k *kubeAPIServer) emptySecret(name string) *corev1.Secret {
	return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: k.seedNamespace}}
}
