// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package connect

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-base/version"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	corebackupbucket "github.com/gardener/gardener/pkg/component/garden/backupbucket"
	"github.com/gardener/gardener/pkg/controller/gardenletdeployer"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/oci"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// NewCommand creates a new cobra.Command.
func NewCommand(globalOpts *cmd.Options) *cobra.Command {
	opts := &Options{Options: globalOpts}

	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Deploy a gardenlet for further cluster management",
		Long:  "Deploy a gardenlet for further cluster management",

		Example: `# Deploy a gardenlet
gardenadm connect`,

		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.ParseArgs(args); err != nil {
				return err
			}

			if err := opts.Validate(); err != nil {
				return err
			}

			if err := opts.Complete(); err != nil {
				return err
			}

			return run(cmd.Context(), opts)
		},
	}

	opts.addFlags(cmd.Flags())

	return cmd
}

func run(ctx context.Context, opts *Options) error {
	opts.Log.Info("Using resources from directory", "configDir", opts.ConfigDir)

	b, err := botanist.NewGardenadmBotanistFromManifests(ctx, opts.Log, nil, opts.ConfigDir, true)
	if err != nil {
		return fmt.Errorf("failed creating gardenadm botanist: %w", err)
	}
	b.SeedClientSet, err = b.CreateClientSet(ctx)
	if err != nil {
		return fmt.Errorf("failed creating client set for self-hosted shoot: %w", err)
	}

	if alreadyConnected, err := isGardenletDeployed(ctx, b); err != nil {
		return fmt.Errorf("failed checking if gardenlet is already deployed: %w", err)
	} else if !alreadyConnected || opts.Force {
		bootstrapClientSet, err := cmd.NewClientSetFromBootstrapToken(opts.ControlPlaneAddress, opts.CertificateAuthority, opts.BootstrapToken, kubernetes.GardenScheme)
		if err != nil {
			return fmt.Errorf("failed creating a new bootstrap garden client set: %w", err)
		}

		var (
			g = flow.NewGraph("connect")

			retrieveShortLivedKubeconfig = g.Add(flow.Task{
				Name: "Retrieving short-lived kubeconfig for garden cluster to prepare Gardener resources",
				Fn: func(ctx context.Context) error {
					gardenClientSet, err := cmd.InitializeTemporaryClientSet(ctx, b, bootstrapClientSet)
					if err != nil {
						return fmt.Errorf("failed retrieving short-lived kubeconfig for garden cluster: %w", err)
					}

					b.Logger.Info("Successfully retrieved short-lived bootstrap kubeconfig for garden cluster")
					b.GardenClient = gardenClientSet.Client()
					return nil
				},
			})
			prepareResources = g.Add(flow.Task{
				Name:         "Preparing Gardener resources in garden cluster",
				Fn:           func(ctx context.Context) error { return prepareGardenerResources(ctx, b) },
				Dependencies: flow.NewTaskIDs(retrieveShortLivedKubeconfig),
			})
			deployGardenlet = g.Add(flow.Task{
				Name: "Deploying gardenlet into self-hosted shoot cluster",
				Fn: func(ctx context.Context) error {
					_, err := newGardenletDeployer(b, bootstrapClientSet).Reconcile(
						ctx,
						b.Logger,
						b.Shoot.GetInfo(),
						nil,
						&seedmanagementv1alpha1.GardenletDeployment{},
						&runtime.RawExtension{Object: &gardenletconfigv1alpha1.GardenletConfiguration{}},
						seedmanagementv1alpha1.BootstrapToken,
						false,
					)
					return err
				},
				Dependencies: flow.NewTaskIDs(prepareResources),
			})
			_ = g.Add(flow.Task{
				Name: "Waiting until gardenlet is ready",
				Fn: func(ctx context.Context) error {
					timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
					defer cancel()
					return retry.Until(timeoutCtx, 2*time.Second, health.IsDeploymentUpdated(b.SeedClientSet.Client(), &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameGardenlet, Namespace: b.Shoot.ControlPlaneNamespace}}))
				},
				Dependencies: flow.NewTaskIDs(deployGardenlet),
			})
		)

		if err := g.Compile().Run(ctx, flow.Opts{
			Log: opts.Log,
		}); err != nil {
			return flow.Errors(err)
		}
	}

	fmt.Fprintf(opts.Out, `
Your self-hosted shoot cluster has successfully been connected to Gardener!

The gardenlet has been deployed in the %s namespace of your self-hosted shoot
cluster and is now taking over the management and lifecycle of it. All
modifications to the Shoot specification should now be performed via the Gardener
API, rather than by directly editing resources in the cluster.

The bootstrap token will be deleted automatically by kube-controller-manager
after it has expired. If you want to delete it right away, run the following
command on any control plane node:

  gardenadm token delete %s

Resources have been successfully synchronized with the garden cluster. You may
(and should) now remove them from the directory at %s, as they will eventually
become outdated:

  rm -rf %[3]s

Happy Gardening!
`, b.Shoot.ControlPlaneNamespace, opts.BootstrapToken, opts.ConfigDir)

	return nil
}

func isGardenletDeployed(ctx context.Context, b *botanist.GardenadmBotanist) (bool, error) {
	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKey{Namespace: b.Shoot.ControlPlaneNamespace, Name: v1beta1constants.DeploymentNameGardenlet}, &appsv1.Deployment{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("failed checking if gardenlet deployment already exists: %w", err)
		}
		return false, nil
	}

	if err := b.SeedClientSet.Client().Get(ctx, client.ObjectKey{Namespace: b.Shoot.ControlPlaneNamespace, Name: gardenletdeployer.GardenletDefaultKubeconfigSecretName}, &corev1.Secret{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("failed checking if gardenlet's kubeconfig secret already exists: %w", err)
		}
		return false, nil
	}

	return true, nil
}

func prepareGardenerResources(ctx context.Context, b *botanist.GardenadmBotanist) error {
	if err := b.GardenClient.Get(ctx, client.ObjectKeyFromObject(b.Resources.CloudProfile), &gardencorev1beta1.CloudProfile{}); err != nil {
		return fmt.Errorf("failed checking for existence of CloudProfile %s (this is not created by 'gardenadm connect' and must exist in the garden cluster): %w", b.Resources.CloudProfile.Name, err)
	}
	b.Logger.Info("CloudProfile existence ensured in garden cluster")

	for _, registration := range b.Resources.ControllerRegistrations {
		if err := b.GardenClient.Get(ctx, client.ObjectKeyFromObject(registration), &gardencorev1beta1.ControllerRegistration{}); err != nil {
			return fmt.Errorf("failed checking for existence of ControllerRegistration %s (this is not created by 'gardenadm connect' and must exist in the garden cluster): %w", registration.Name, err)
		}
	}
	b.Logger.Info("ControllerRegistration existences ensured in garden cluster")

	for _, deployment := range b.Resources.ControllerDeployments {
		if err := b.GardenClient.Get(ctx, client.ObjectKeyFromObject(deployment), &gardencorev1.ControllerDeployment{}); err != nil {
			return fmt.Errorf("failed checking for existence of ControllerDeployment %s (this is not created by 'gardenadm connect' and must exist in the garden cluster): %w", deployment.Name, err)
		}
	}
	b.Logger.Info("ControllerDeployments existences ensured in garden cluster")

	// We do not handle Project using 'garden' namespace because gardener-apiserver defaults .spec.tolerations for this
	// Project. This requires special permissions for a custom verb that we do not want to grant to the gardenadm user
	// for self-hosted shoots. Since this is a special project anyway, it must have been created beforehand.
	if project := b.Resources.Project.DeepCopy(); ptr.Deref(project.Spec.Namespace, "") != v1beta1constants.GardenNamespace {
		if err := b.GardenClient.Create(ctx, project); client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed creating Project resource %s in garden cluster: %w", project.Name, err)
		}
		b.Logger.Info("Project resource ensured in garden cluster")
	}

	for _, configMap := range b.Resources.ConfigMaps {
		if err := b.GardenClient.Create(ctx, configMap.DeepCopy()); client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed creating ConfigMap resource %s in garden cluster: %w", client.ObjectKeyFromObject(configMap), err)
		}
	}
	b.Logger.Info("ConfigMap resources ensured in garden cluster")

	for _, secret := range b.Resources.Secrets {
		if err := b.GardenClient.Create(ctx, secret.DeepCopy()); client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed creating Secret resource %s in garden cluster: %w", client.ObjectKeyFromObject(secret), err)
		}
	}
	b.Logger.Info("Secret resources ensured in garden cluster")

	for _, workloadIdentity := range b.Resources.WorkloadIdentities {
		if err := b.GardenClient.Create(ctx, workloadIdentity.DeepCopy()); client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed creating WorkloadIdentity resource %s in garden cluster: %w", client.ObjectKeyFromObject(workloadIdentity), err)
		}
	}
	b.Logger.Info("WorkloadIdentity resources ensured in garden cluster")

	if b.Resources.SecretBinding != nil {
		if err := b.GardenClient.Create(ctx, b.Resources.SecretBinding.DeepCopy()); client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed creating SecretBinding resource %s in garden cluster: %w", client.ObjectKeyFromObject(b.Resources.SecretBinding), err)
		}
		b.Logger.Info("SecretBinding resource ensured in garden cluster")
	}

	if b.Resources.CredentialsBinding != nil {
		if err := b.GardenClient.Create(ctx, b.Resources.CredentialsBinding.DeepCopy()); client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed creating CredentialsBinding resource %s in garden cluster: %w", client.ObjectKeyFromObject(b.Resources.CredentialsBinding), err)
		}
		b.Logger.Info("CredentialsBinding resource ensured in garden cluster")
	}

	shoot := b.Shoot.GetInfo().DeepCopy()
	shoot.Status = gardencorev1beta1.ShootStatus{} // we don't want to copy the in-memory status, otherwise we cannot compute a patch further below
	if err := b.GardenClient.Create(ctx, shoot); client.IgnoreAlreadyExists(err) != nil {
		return fmt.Errorf("failed creating Shoot resource %s in garden cluster: %w", client.ObjectKeyFromObject(b.Shoot.GetInfo()), err)
	}

	patch := client.MergeFrom(shoot.DeepCopy())
	shoot.Status = b.Shoot.GetInfo().Status
	if err := b.GardenClient.Status().Patch(ctx, shoot, patch); err != nil {
		return fmt.Errorf("failed patching Shoot %s status in garden cluster: %w", client.ObjectKeyFromObject(b.Shoot.GetInfo()), err)
	}
	b.Shoot.SetInfo(shoot)
	b.Logger.Info("Shoot resource ensured in garden cluster")

	// TODO(rfranzke): Remove this from here once the `gardenlet` runs the `shoot/shoot` reconciler (which will also
	//  create/reconcile the Backup{Bucket,Entry} resources).
	if v1beta1helper.GetBackupConfigForShoot(b.Shoot.GetInfo(), nil) != nil {
		if err := corebackupbucket.New(b.Logger, b.GardenClient, &corebackupbucket.Values{
			Name:          string(b.Shoot.GetInfo().Status.UID),
			Config:        v1beta1helper.GetBackupConfigForShoot(b.Shoot.GetInfo(), nil),
			DefaultRegion: b.Shoot.GetInfo().Spec.Region,
			Clock:         b.Clock,
			Shoot:         b.Shoot.GetInfo(),
		}, corebackupbucket.DefaultInterval, corebackupbucket.DefaultTimeout).Deploy(ctx); err != nil {
			return fmt.Errorf("failed reconciling core.gardener.cloud/v1beta1.BackupBucket resource: %w", err)
		}
		b.Logger.Info("BackupBucket resource ensured in garden cluster")

		if err := b.DefaultCoreBackupEntry().Deploy(ctx); err != nil {
			return fmt.Errorf("failed creating core.gardener.cloud/v1beta1.BackupEntry resource in garden cluster: %w", err)
		}
		b.Logger.Info("BackupEntry resource ensured in garden cluster")
	}

	return nil
}

func newGardenletDeployer(b *botanist.GardenadmBotanist, gardenClientSet kubernetes.Interface) gardenletdeployer.Interface {
	return &gardenletdeployer.Actuator{
		GardenConfig:        gardenClientSet.RESTConfig(),
		GardenClient:        gardenClientSet.Client(),
		GetTargetClientFunc: func(_ context.Context) (kubernetes.Interface, error) { return b.SeedClientSet, nil },
		CheckIfVPAAlreadyExists: func(_ context.Context) (bool, error) {
			return false, nil
		},
		GetTargetDomain: func() string {
			return ""
		},
		ApplyGardenletChart: func(ctx context.Context, targetChartApplier kubernetes.ChartApplier, values map[string]interface{}) error {
			gardenletChartImage, err := imagevector.Charts().FindImage(imagevector.ChartImageNameGardenlet)
			if err != nil {
				return err
			}
			gardenletChartImage.WithOptionalTag(version.Get().GitVersion)

			archive, err := oci.NewHelmRegistry(b.GardenClient).Pull(ctx, &gardencorev1.OCIRepository{Ref: ptr.To(gardenletChartImage.String())})
			if err != nil {
				return fmt.Errorf("failed pulling Helm chart %s from OCI repository: %w", gardenletChartImage.String(), err)
			}

			return targetChartApplier.ApplyFromArchive(ctx, archive, b.Shoot.ControlPlaneNamespace, "gardenlet", kubernetes.Values(values))
		},
		Clock:                    clock.RealClock{},
		ValuesHelper:             gardenletdeployer.NewValuesHelper(nil),
		Recorder:                 &record.FakeRecorder{},
		GardenletNamespaceTarget: b.Shoot.ControlPlaneNamespace,
		BootstrapToken:           gardenClientSet.RESTConfig().BearerToken,
	}
}
