// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// Values is a set of configuration values for the Etcd component.
type Values struct {
	// Image is the container image used for Etcd.
	Image string
}

// New creates a new instance of DeployWaiter for the Etcd.
func New(
	client client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) component.Deployer {
	return &etcdDeployer{
		client:         client,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
	}
}

type etcdDeployer struct {
	client         client.Client
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
}

func (e *etcdDeployer) Deploy(_ context.Context) error {
	return nil
}

func (e *etcdDeployer) Destroy(_ context.Context) error {
	return nil
}
