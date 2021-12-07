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
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils/flow"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// fillTheVoid is a constant used as a dummy value during deletion
	fillTheVoid = "dummy"
)

// Delete runs the delete operation.
func (o *operation) Delete(ctx context.Context) error {
	var (
		graph = flow.NewGraph("Gardener ControlPlane Deletion")

		// refuse deletion of a Gardener landscape with existing Seed clusters
		// Seeds can only be removed if there are not more Shoot clusters deployed.
		safetyCheck = graph.Add(flow.Task{
			Name: "Safety Check",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				seedList := &gardencorev1beta1.SeedList{}
				if err := o.getGardenClient().Client().List(ctx, seedList); err != nil {
					return fmt.Errorf("failed to check if Gardener installation is ready to be deleted: %v", err)
				}

				if len(seedList.Items) > 0 {
					return fmt.Errorf("refusing to delete a Gardener installation with remaining Seed clusters. To delete a Seed cluster the corresponding Shoot clusters have to be deleted first")
				}

				o.log.Info("Safety check passed. Go for deletion.")
				return nil
			}),
		})

		fillValues = graph.Add(flow.Task{
			Name: "Fill required helm chart values for deletion",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				if o.imports.VirtualGarden != nil && o.imports.VirtualGarden.Enabled {
					// we do not need the real kubeconfigs for the control plane component - but they should be set so that the respective secrets are deleted
					o.VirtualGardenKubeconfigGardenerAPIServer = pointer.String(fillTheVoid)
					o.VirtualGardenKubeconfigGardenerControllerManager = pointer.String(fillTheVoid)
					o.VirtualGardenKubeconfigGardenerScheduler = pointer.String(fillTheVoid)
					o.VirtualGardenKubeconfigGardenerAdmissionController = pointer.String(fillTheVoid)
				}

				o.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt = pointer.String(fillTheVoid)
				o.imports.GardenerAPIServer.ComponentConfiguration.CA.Key = pointer.String(fillTheVoid)
				o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Crt = pointer.String(fillTheVoid)
				o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Key = pointer.String(fillTheVoid)

				o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt = pointer.String(fillTheVoid)
				o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Key = pointer.String(fillTheVoid)
				o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt = pointer.String(fillTheVoid)
				o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Key = pointer.String(fillTheVoid)

				if o.imports.GardenerAdmissionController.Enabled {
					o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt = pointer.String(fillTheVoid)
					o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Key = pointer.String(fillTheVoid)
				}

				o.imports.Identity = pointer.String(fillTheVoid)
				o.imports.GardenerAPIServer.ComponentConfiguration.Encryption = &apiserverconfigv1.EncryptionConfiguration{}
				o.imports.OpenVPNDiffieHellmanKey = pointer.String(fillTheVoid)

				return nil
			}),
			Dependencies: flow.NewTaskIDs(safetyCheck),
		})

		// need to delete webhooks from the Garden cluster that block the deletion of Gardener secrets
		deleteWebhooks = graph.Add(flow.Task{
			Name: "Deleting webhook configurations from the Garden cluster",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				validating := &admissionregistrationv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: validatingWebhookNameValidateNamespaceDeletion,
					},
				}
				if err := client.IgnoreNotFound(o.getGardenClient().Client().Delete(ctx, validating)); err != nil {
					return err
				}

				return nil
			}),
			Dependencies: flow.NewTaskIDs(safetyCheck),
		})

		destroyApplicationChart = graph.Add(flow.Task{
			Name: "Destroying the application chart in the Garden cluster",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				if err := o.DestroyApplicationChart(ctx); err != nil {
					return err
				}
				return nil
			}),
			Dependencies: flow.NewTaskIDs(fillValues, deleteWebhooks),
		})

		_ = graph.Add(flow.Task{
			Name: "Destroying CA private key secrets in the runtime cluster",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				apiServerCA := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretNameLandscaperGardenerAPIServerKey,
						Namespace: v1beta1constants.GardenNamespace,
					},
				}
				if err := client.IgnoreNotFound(o.runtimeClient.Client().Delete(ctx, apiServerCA)); err != nil {
					return err
				}

				admissionServerCA := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretNameLandscaperGardenerAdmissionControllerKey,
						Namespace: v1beta1constants.GardenNamespace,
					},
				}
				if err := client.IgnoreNotFound(o.runtimeClient.Client().Delete(ctx, admissionServerCA)); err != nil {
					return err
				}

				return nil
			}),
			Dependencies: flow.NewTaskIDs(destroyApplicationChart),
		})

		_ = graph.Add(flow.Task{
			Name: "Destroying the runtime chart",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				if err := o.DestroyRuntimeChart(ctx); err != nil {
					return err
				}
				return nil
			}),
			Dependencies: flow.NewTaskIDs(destroyApplicationChart),
		})
	)

	return graph.Compile().Run(ctx, flow.Opts{
		Logger:           o.log,
		ProgressReporter: flow.NewImmediateProgressReporter(o.progressReporter),
	})
}
