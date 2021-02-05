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

package deployment_test

import (
	"bytes"
	"context"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func expectOIDCCABundle(ctx context.Context, valuesProvider KubeAPIServerValuesProvider) *string {
	if valuesProvider.GetAPIServerConfig() == nil || valuesProvider.GetAPIServerConfig().OIDCConfig == nil {
		return nil
	}

	mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver-oidc-cabundle"), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))

	expectedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver-oidc-cabundle",
			Namespace: defaultSeedNamespace,
		},
		Data: map[string][]byte{
			"ca.crt": []byte(utils.EncodeBase64([]byte(*valuesProvider.GetAPIServerConfig().OIDCConfig.CABundle))),
		},
	}

	mockSeedClient.EXPECT().Create(ctx, expectedSecret).Times(1)

	encoder := kubernetes.SeedCodec.EncoderForVersion(kubernetes.SeedSerializer, corev1.SchemeGroupVersion)
	buf := new(bytes.Buffer)
	err := encoder.Encode(expectedSecret, buf)
	Expect(err).ToNot(HaveOccurred())
	sha256Hex := utils.ComputeSHA256Hex(buf.Bytes())
	return &sha256Hex
}

func expectServiceAccountSigningKey(ctx context.Context, valuesProvider KubeAPIServerValuesProvider) *string {
	if valuesProvider.GetAPIServerConfig() == nil || valuesProvider.GetAPIServerConfig().ServiceAccountConfig == nil || valuesProvider.GetAPIServerConfig().ServiceAccountConfig.SigningKeySecret == nil {
		return nil
	}
	keyContent := []byte("dummy-key")

	mockGardenClient.EXPECT().Get(ctx, kutil.Key(defaultGardenNamespace, valuesProvider.GetAPIServerConfig().ServiceAccountConfig.SigningKeySecret.Name), gomock.AssignableToTypeOf(&corev1.Secret{})).
		DoAndReturn(func(ctx context.Context, key client.ObjectKey, obj *corev1.Secret) error {
			*obj = corev1.Secret{Data: map[string][]byte{
				"signing-key": keyContent,
			}}
			return nil
		})

	mockSeedClient.EXPECT().Get(ctx, kutil.Key(defaultSeedNamespace, "kube-apiserver-service-account-signing-key"), gomock.AssignableToTypeOf(&corev1.Secret{})).Return(apierrors.NewNotFound(schema.GroupResource{}, "foo"))

	// expect creation of secret in the Seed cluster
	expectedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver-service-account-signing-key",
			Namespace: defaultSeedNamespace,
		},
		Data: map[string][]byte{
			"signing-key": []byte(utils.EncodeBase64(keyContent)),
		},
	}

	mockSeedClient.EXPECT().Create(ctx, expectedSecret).Times(1)

	encoder := kubernetes.SeedCodec.EncoderForVersion(kubernetes.SeedSerializer, corev1.SchemeGroupVersion)
	buf := new(bytes.Buffer)
	err := encoder.Encode(expectedSecret, buf)
	Expect(err).ToNot(HaveOccurred())
	sha256Hex := utils.ComputeSHA256Hex(buf.Bytes())
	return &sha256Hex
}
