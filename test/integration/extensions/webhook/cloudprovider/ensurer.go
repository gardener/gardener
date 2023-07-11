// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package cloudprovider

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener/extensions/pkg/webhook/cloudprovider"
	extensionscontextwebhook "github.com/gardener/gardener/extensions/pkg/webhook/context"
)

// NewEnsurer creates cloudprovider ensurer.
func NewEnsurer(logger logr.Logger) cloudprovider.Ensurer {
	return &ensurer{
		logger: logger,
	}
}

type ensurer struct {
	logger logr.Logger
}

// EnsureCloudProviderSecret is implemented on extension side which mutates the cloudprovider secret. contain
// For testing purpose we are mutating the cloudprovider secret's data to check whether this
// function is called in webhook.
func (e *ensurer) EnsureCloudProviderSecret(ctx context.Context, _ extensionscontextwebhook.GardenContext, new, _ *corev1.Secret) error {
	e.logger.Info("Mutate cloudprovider secret", "namespace", new.Namespace, "name", new.Name)
	new.Data["clientID"] = []byte(`foo`)
	new.Data["clientSecret"] = []byte(`bar`)

	return nil
}
