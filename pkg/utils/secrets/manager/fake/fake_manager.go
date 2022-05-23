// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package fake

import (
	"context"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagerIdentity is the fake secret manager's identity.
const ManagerIdentity = "fake"

type fakeManager struct {
	client    client.Client
	namespace string
}

var _ secretsmanager.Interface = &fakeManager{}

// New returns a simple implementation of secretsmanager.Interface which can be used to fake the SecretsManager in unit
// tests.
func New(client client.Client, namespace string) *fakeManager {
	return &fakeManager{
		client:    client,
		namespace: namespace,
	}
}

func (m *fakeManager) Get(name string, opts ...secretsmanager.GetOption) (*corev1.Secret, bool) {
	options := &secretsmanager.GetOptions{}
	options.ApplyOptions(opts)

	secretName := name
	if options.Class != nil {
		secretName += "-" + string(*options.Class)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: m.namespace,
		},
	}
	if err := m.client.Get(context.TODO(), client.ObjectKeyFromObject(secret), secret); err != nil {
		return nil, false
	}

	if secret.Data == nil {
		secret.Data = make(map[string][]byte, 1)
	}
	secret.Data["data-for"] = []byte(name)

	return secret, true
}

func (m *fakeManager) Generate(ctx context.Context, config secretutils.ConfigInterface, opts ...secretsmanager.GenerateOption) (*corev1.Secret, error) {
	options := &secretsmanager.GenerateOptions{}
	if err := options.ApplyOptions(m, config, opts); err != nil {
		return nil, err
	}

	objectMeta, err := secretsmanager.ObjectMeta(m.namespace, ManagerIdentity, config, true, "", nil, &options.Persist, nil)
	if err != nil {
		return nil, err
	}

	data, err := config.Generate()
	if err != nil {
		return nil, err
	}

	objectMeta.Labels["rotation-strategy"] = string(options.RotationStrategy)
	secret := secretsmanager.Secret(objectMeta, data.SecretData())

	if err := m.client.Create(ctx, secret); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, err
		}

		secret = &corev1.Secret{}
		if err := m.client.Get(ctx, kutil.Key(objectMeta.Namespace, objectMeta.Name), secret); err != nil {
			return nil, err
		}

		patch := client.MergeFrom(secret.DeepCopy())
		secret.Labels = objectMeta.Labels
		secret.Immutable = pointer.Bool(true)
		if err := m.client.Patch(ctx, secret, patch); err != nil {
			return nil, err
		}
	}

	return secret, nil
}

func (m *fakeManager) Cleanup(_ context.Context) error {
	return nil
}
