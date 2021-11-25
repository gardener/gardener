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
	"time"

	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/exports"
	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/etcdencryption"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Reconcile runs the reconcile operation.
func (o *operation) Reconcile(ctx context.Context) (*exports.Exports, bool, bool, error) {
	var (
		graph = flow.NewGraph("Gardener ControlPlane Reconciliation")

		fetchSecretReferences = graph.Add(flow.Task{
			Name: "Fetching import secret references",
			Fn:   flow.TaskFn(o.FetchAndValidateConfigurationFromSecretReferences).SkipIf(o.imports.CertificateRotation != nil && o.imports.CertificateRotation.Rotate),
		})

		getIdentity = graph.Add(flow.Task{
			Name: "Getting Gardener Identity",
			Fn:   flow.TaskFn(o.GetIdentity).DoIf(o.imports.Identity == nil),
		})

		prepareGardenNamespace = graph.Add(flow.Task{
			Name: "Preparing Garden namespace",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				client := o.getGardenClient().Client()

				gardenNamespace := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: gardencorev1beta1constants.GardenNamespace,
					},
				}

				// create + label garden namespace
				if _, err := controllerutils.CreateOrGetAndMergePatch(ctx, client, gardenNamespace, func() error {
					kutil.SetMetaDataLabel(&gardenNamespace.ObjectMeta, gardencorev1beta1constants.GardenRole, gardencorev1beta1constants.GardenRoleProject)
					kutil.SetMetaDataLabel(&gardenNamespace.ObjectMeta, gardencorev1beta1constants.ProjectName, gardencorev1beta1constants.GardenRoleProject)
					kutil.SetMetaDataLabel(&gardenNamespace.ObjectMeta, gardencorev1beta1constants.LabelApp, "gardener")
					return nil
				}); err != nil {
					return err
				}
				return nil
			}),
		})

		getDiffieHellmann = graph.Add(flow.Task{
			Name: "Get OpenVPN Diffie-Hellmann Key",
			Fn:   flow.TaskFn(o.GetOrDefaultDiffieHellmannKey).DoIf(o.imports.OpenVPNDiffieHellmanKey == nil),
		})

		syncExistingInstallation = graph.Add(flow.Task{
			Name:         "Syncing with existing Gardener Installation",
			Fn:           flow.TaskFn(o.SyncWithExistingGardenerInstallation).SkipIf(o.imports.CertificateRotation != nil && o.imports.CertificateRotation.Rotate),
			Dependencies: flow.NewTaskIDs(fetchSecretReferences),
		})

		generateEtcdEncryption = graph.Add(flow.Task{
			Name:         "Generating etcd encryption configuration",
			Fn:           flow.TaskFn(o.GenerateEncryptionConfiguration).DoIf(o.imports.GardenerAPIServer.ComponentConfiguration.Encryption == nil),
			Dependencies: flow.NewTaskIDs(syncExistingInstallation),
		})

		checkExpiringCertificates = graph.Add(flow.Task{
			Name:         "Check for expiring certificates",
			Fn:           flow.TaskFn(o.CheckForExpiringCertificates).SkipIf(o.imports.CertificateRotation != nil && o.imports.CertificateRotation.Rotate),
			Dependencies: flow.NewTaskIDs(syncExistingInstallation),
		})

		prepareCompleteRotation = graph.Add(flow.Task{
			Name:         "Prepare complete certificate rotation",
			Fn:           flow.TaskFn(o.PrepareCompleteCertificateRotation).DoIf(o.imports.CertificateRotation != nil && o.imports.CertificateRotation.Rotate),
			Dependencies: flow.NewTaskIDs(syncExistingInstallation),
		})

		generateCAs = graph.Add(flow.Task{
			Name:         "Generating missing Certificate Authorities",
			Fn:           o.GenerateCACertificates,
			Dependencies: flow.NewTaskIDs(checkExpiringCertificates, prepareCompleteRotation),
		})

		generateAPIServerCerts = graph.Add(flow.Task{
			Name:         "Generating API server certificates",
			Fn:           o.GenerateAPIServerCertificates,
			Dependencies: flow.NewTaskIDs(generateCAs),
		})

		generateGACCerts = graph.Add(flow.Task{
			Name:         "Generating Admission controller certificates",
			Fn:           flow.TaskFn(o.GenerateAdmissionControllerCertificates).DoIf(o.imports.GardenerAdmissionController.Enabled),
			Dependencies: flow.NewTaskIDs(generateCAs),
		})

		generateGCMCerts = graph.Add(flow.Task{
			Name:         "Generating Controller Manager certificates",
			Fn:           o.GenerateControllerManagerCertificates,
			Dependencies: flow.NewTaskIDs(generateCAs),
		})

		// Execute right before charts are deployed
		// Reason: avoid deletion of gardenlet RBAC roles and then abort operation due to some minor error condition in a parallel flow
		// => avoid potentially catastrophic state where no Gardenlet can talk to the garden cluster
		seedAuthorizerPreparation = graph.Add(flow.Task{
			Name: "if seed authorizer is enabled: delete ClusterRole{Binding}s allowing cluster-admin access for gardenlets",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				binding := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name: gardencorev1beta1constants.SeedsGroup,
					},
				}
				clusterRole := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: gardencorev1beta1constants.SeedsGroup,
					},
				}
				if err := o.getGardenClient().Client().Delete(ctx, binding); client.IgnoreNotFound(err) != nil {
					return fmt.Errorf("failed to delete clusterrole binding %q to remove access for gardenlets: %w", gardencorev1beta1constants.SeedsGroup, err)
				}

				if err := o.getGardenClient().Client().Delete(ctx, clusterRole); client.IgnoreNotFound(err) != nil {
					return fmt.Errorf("failed to delete clusterrole %q to remove access for gardenlets: %w", gardencorev1beta1constants.SeedsGroup, err)
				}
				return nil
			}).DoIf(o.imports.Rbac != nil && o.imports.Rbac.SeedAuthorizer != nil && *o.imports.Rbac.SeedAuthorizer.Enabled),
			Dependencies: flow.NewTaskIDs(prepareGardenNamespace, generateAPIServerCerts, generateGACCerts, generateGCMCerts, generateEtcdEncryption, getDiffieHellmann, getIdentity),
		})

		deployApplicationChart = graph.Add(flow.Task{
			Name: "Deploying the application chart into the Garden cluster",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				if err := o.DeployApplicationChart(ctx); err != nil {
					return err
				}

				o.successfullyDeployedApplicationChart = true
				return nil
			}),
			Dependencies: flow.NewTaskIDs(seedAuthorizerPreparation),
		})

		getVirtualGardenClusterEndpoint = graph.Add(flow.Task{
			Name:         "Determining Virtual Garden cluster endpoint",
			Fn:           flow.TaskFn(o.GetVirtualGardenClusterEndpoint).DoIf(o.imports.VirtualGarden != nil && o.imports.VirtualGarden.Enabled),
			Dependencies: flow.NewTaskIDs(deployApplicationChart),
		})

		generateKubeconfigGardenerAPIServer = graph.Add(flow.Task{
			Name: "Generate kubeconfig for the Gardener API Server",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				var err error
				if o.VirtualGardenKubeconfigGardenerAPIServer, err = o.GenerateVirtualGardenKubeconfig(ctx, deploymentNameGardenerAPIServer); err != nil {
					return err
				}
				return nil
			}).DoIf(o.imports.VirtualGarden != nil && o.imports.VirtualGarden.Enabled),
			Dependencies: flow.NewTaskIDs(getVirtualGardenClusterEndpoint),
		})

		generateKubeconfigGardenerControllerManager = graph.Add(flow.Task{
			Name: "Generate kubeconfig for the Gardener Controller Manager",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				var err error
				if o.VirtualGardenKubeconfigGardenerControllerManager, err = o.GenerateVirtualGardenKubeconfig(ctx, deploymentNameGardenerControllerManager); err != nil {
					return err
				}
				return nil
			}).DoIf(o.imports.VirtualGarden != nil && o.imports.VirtualGarden.Enabled),
			Dependencies: flow.NewTaskIDs(getVirtualGardenClusterEndpoint),
		})

		generateKubeconfigGardenerScheduler = graph.Add(flow.Task{
			Name: "Generate kubeconfig for the Gardener Scheduler",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				var err error
				if o.VirtualGardenKubeconfigGardenerScheduler, err = o.GenerateVirtualGardenKubeconfig(ctx, deploymentNameGardenerScheduler); err != nil {
					return err
				}
				return nil
			}).DoIf(o.imports.VirtualGarden != nil && o.imports.VirtualGarden.Enabled),
			Dependencies: flow.NewTaskIDs(getVirtualGardenClusterEndpoint),
		})

		generateKubeconfigGardenerAdmissionController = graph.Add(flow.Task{
			Name: "Generate kubeconfig for the Gardener Admission Controller",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				var err error
				if o.VirtualGardenKubeconfigGardenerAdmissionController, err = o.GenerateVirtualGardenKubeconfig(ctx, deploymentNameGardenerAdmissionController); err != nil {
					return err
				}
				return nil
			}).DoIf(o.imports.VirtualGarden != nil && o.imports.VirtualGarden.Enabled && o.imports.GardenerAdmissionController.Enabled),
			Dependencies: flow.NewTaskIDs(getVirtualGardenClusterEndpoint),
		})

		deployRuntimeChart = graph.Add(flow.Task{
			Name: "Deploying the runtime chart",
			Fn: flow.TaskFn(func(ctx context.Context) error {
				// TODO

				o.successfullyDeployedRuntimeChart = true
				return nil
			}),
			Dependencies: flow.NewTaskIDs(deployApplicationChart, generateKubeconfigGardenerAPIServer, generateKubeconfigGardenerControllerManager, generateKubeconfigGardenerScheduler, generateKubeconfigGardenerAdmissionController),
		})

		updateSecretReferences = graph.Add(flow.Task{
			Name:         "Update secret references with certificates",
			Fn:           flow.TaskFn(o.UpdateSecretReferences).RetryUntilTimeout(5*time.Second, 5*time.Minute),
			Dependencies: flow.NewTaskIDs(deployRuntimeChart),
		})

		_ = graph.Add(flow.Task{
			Name:         "Verifying Control Plane setup",
			Fn:           o.VerifyControlplane,
			Dependencies: flow.NewTaskIDs(updateSecretReferences),
		})
	)

	err := graph.Compile().Run(ctx, flow.Opts{
		Logger:           o.log,
		ProgressReporter: flow.NewImmediateProgressReporter(o.progressReporter),
	})

	return o.getExports(), o.successfullyDeployedApplicationChart, o.successfullyDeployedRuntimeChart, err
}

// getExports sets the exports based on the import configuration that contains pre-configured and generated certificates
func (o *operation) getExports() *exports.Exports {
	// required certificates
	o.exports.GardenerIdentity = *o.imports.Identity
	o.exports.OpenVPNDiffieHellmanKey = *o.imports.OpenVPNDiffieHellmanKey

	encryption, err := etcdencryption.Write(o.imports.GardenerAPIServer.ComponentConfiguration.Encryption)
	if err != nil {
		o.log.Warnf("failed to marshal encryption configuration. Will not be exported. However, it is deployed as a secret %s%s in the runtime cluster", gardencorev1beta1constants.GardenNamespace, secretNameGardenerEncryptionConfig)
	} else {
		o.exports.GardenerAPIServerEncryptionConfiguration = string(encryption)
	}

	o.exports.GardenerAPIServerCA.Crt = *o.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt
	o.exports.GardenerAPIServerTLSServing.Crt = *o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt
	o.exports.GardenerAPIServerTLSServing.Key = *o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Key

	o.exports.GardenerControllerManagerTLSServing.Crt = *o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt
	o.exports.GardenerControllerManagerTLSServing.Key = *o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Key

	// optional
	if o.imports.GardenerAPIServer.ComponentConfiguration.CA.Key != nil {
		o.exports.GardenerAPIServerCA.Key = *o.imports.GardenerAPIServer.ComponentConfiguration.CA.Key
	}

	if o.imports.GardenerAdmissionController != nil && o.imports.GardenerAdmissionController.Enabled {
		// safely dereference, because the CA must have been either provided or generated during the reconciliation
		o.exports.GardenerAdmissionControllerCA.Crt = *o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Crt

		if o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Key != nil {
			o.exports.GardenerAdmissionControllerCA.Key = *o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Key
		}

		o.exports.GardenerAdmissionControllerTLSServing.Crt = *o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt
		o.exports.GardenerAdmissionControllerTLSServing.Key = *o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Key
	}

	return &o.exports
}
