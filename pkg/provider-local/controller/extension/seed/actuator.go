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

package seed

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/gardener/gardener/extensions/pkg/controller/extension"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// ApplicationName is the name of the application.
	ApplicationName string = "local-ext-seed"
	// ManagedResourceNamesSeed is the name used to describe the managed seed resources.
	ManagedResourceNamesSeed string = ApplicationName
)

type actuator struct {
	client client.Client
}

// NewActuator returns an actuator responsible for Extension resources.
func NewActuator(mgr manager.Manager) extension.Actuator {
	return &actuator{
		client: mgr.GetClient(),
	}
}

// Reconcile the extension resource.
func (a *actuator) Reconcile(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	namespace := ex.Namespace
	seedResources, err := getSeedResources(namespace)
	if err != nil {
		return err
	}

	if err := managedresources.CreateForSeed(ctx, a.client, namespace, ManagedResourceNamesSeed, false, seedResources); err != nil {
		return err
	}

	twoMinutes := 2 * time.Minute
	timeoutSeedCtx, cancelSeedCtx := context.WithTimeout(ctx, twoMinutes)
	defer cancelSeedCtx()
	return managedresources.WaitUntilHealthy(timeoutSeedCtx, a.client, namespace, ManagedResourceNamesSeed)
}

// Delete the extension resource.
func (a *actuator) Delete(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	namespace := ex.GetNamespace()
	twoMinutes := 2 * time.Minute

	timeoutSeedCtx, cancelSeedCtx := context.WithTimeout(ctx, twoMinutes)
	defer cancelSeedCtx()

	if err := managedresources.DeleteForSeed(ctx, a.client, namespace, ManagedResourceNamesSeed); err != nil {
		return err
	}

	return managedresources.WaitUntilDeleted(timeoutSeedCtx, a.client, namespace, ManagedResourceNamesSeed)
}

// Migrate the extension resource.
func (a *actuator) Migrate(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	return a.Delete(ctx, log, ex)
}

// Restore the extension resource.
func (a *actuator) Restore(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	return a.Reconcile(ctx, log, ex)
}

func getLabels() map[string]string {
	return map[string]string{
		"app.kubernetes.io/name": ApplicationName,
	}
}

func getSeedResources(namespace string) (map[string][]byte, error) {
	registry := managedresources.NewRegistry(kubernetesclient.SeedScheme, kubernetesclient.SeedCodec, kubernetesclient.SeedSerializer)
	return registry.AddAllAndSerialize(
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ApplicationName,
				Namespace: namespace,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		},
	)
}
