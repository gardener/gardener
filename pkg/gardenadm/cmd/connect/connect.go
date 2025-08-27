// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package connect

import (
	"context"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/component-base/version"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controller/gardenletdeployer"
	"github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenadm/cmd"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/kubernetes/certificatesigningrequest"
	"github.com/gardener/gardener/pkg/utils/oci"
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

	b, err := botanist.NewAutonomousBotanistFromManifests(ctx, opts.Log, nil, opts.ConfigDir, true)
	if err != nil {
		return fmt.Errorf("failed creating autonomous botanist: %w", err)
	}
	b.SeedClientSet, err = b.CreateClientSet(ctx)
	if err != nil {
		return fmt.Errorf("failed creating client set for autonomous shoot: %w", err)
	}

	// TODO(rfranzke): Skip this if gardenlet is already connected (see subsequent commit).
	bootstrapClientSet, err := kubernetes.NewWithConfig(kubernetes.WithRESTConfig(&rest.Config{
		Host:            opts.ControlPlaneAddress,
		TLSClientConfig: rest.TLSClientConfig{CAData: opts.CertificateAuthority},
		BearerToken:     opts.BootstrapToken,
	}), kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.GardenScheme}), kubernetes.WithDisabledCachedClient())
	if err != nil {
		return fmt.Errorf("failed creating garden client set: %w", err)
	}

	var (
		g = flow.NewGraph("connect")

		retrieveShortLivedKubeconfig = g.Add(flow.Task{
			Name: "Retrieving short-lived kubeconfig for garden cluster to prepare Gardener resources",
			Fn: func(ctx context.Context) error {
				return requestShortLivedClientCertificateForGarden(ctx, b, bootstrapClientSet)
			},
		})
		prepareGardenerResources = g.Add(flow.Task{
			Name: "Preparing Gardener resources in garden cluster",
			Fn: func(ctx context.Context) error {
				// TODO(rfranzke): Dummy call with garden client - replaced in a subsequent commit with actual logic.
				return b.GardenClient.List(ctx, &corev1.NodeList{})
			},
			Dependencies: flow.NewTaskIDs(retrieveShortLivedKubeconfig),
		})
		_ = g.Add(flow.Task{
			Name: "Deploying gardenlet into autonomous shoot cluster",
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
			Dependencies: flow.NewTaskIDs(prepareGardenerResources),
		})
	)

	if err := g.Compile().Run(ctx, flow.Opts{
		Log: opts.Log,
	}); err != nil {
		return flow.Errors(err)
	}

	fmt.Fprintf(opts.Out, `
Your autonomous shoot cluster has successfully been connected to Gardener!

The gardenlet has been deployed in the %s namespace of your autonomous shoot
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
`, b.Shoot.ControlPlaneNamespace, opts.BootstrapToken, opts.ConfigDir)

	return nil
}

func requestShortLivedClientCertificateForGarden(ctx context.Context, b *botanist.AutonomousBotanist, bootstrapClientSet kubernetes.Interface) error {
	certificateSubject := &pkix.Name{
		Organization: []string{v1beta1constants.ShootsGroup},
		CommonName:   v1beta1constants.GardenadmUserNamePrefix + b.Shoot.GetInfo().Namespace + ":" + b.Shoot.GetInfo().Name,
	}

	certData, privateKeyData, _, err := certificatesigningrequest.RequestCertificate(ctx, b.Logger, bootstrapClientSet.Kubernetes(), certificateSubject, []string{}, []net.IP{}, &metav1.Duration{Duration: 10 * time.Minute}, "gardenadm-connect-csr-")
	if err != nil {
		return fmt.Errorf("unable to bootstrap the kubeconfig for the Garden cluster: %w", err)
	}

	kubeconfig, err := gardenletbootstraputil.CreateGardenletKubeconfigWithClientCertificate(bootstrapClientSet.RESTConfig(), privateKeyData, certData)
	if err != nil {
		return fmt.Errorf("failed creating short-lived kubeconfig with the retrieved client certificate: %w", err)
	}

	gardenClientSet, err := kubernetes.NewClientFromBytes(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed creating garden client set from short-lived kubeconfig: %w", err)
	}

	b.Logger.Info("Successfully retrieved short-lived kubeconfig for garden cluster")
	b.GardenClient = gardenClientSet.Client()
	return nil
}

func newGardenletDeployer(b *botanist.AutonomousBotanist, gardenClientSet kubernetes.Interface) gardenletdeployer.Interface {
	return &gardenletdeployer.Actuator{
		GardenConfig:        gardenClientSet.RESTConfig(),
		GardenClient:        gardenClientSet.Client(),
		GetTargetClientFunc: func(_ context.Context) (kubernetes.Interface, error) { return b.SeedClientSet, nil },
		CheckIfVPAAlreadyExists: func(_ context.Context) (bool, error) {
			return false, nil
		},
		GetInfrastructureSecret: func(_ context.Context) (*corev1.Secret, error) { return nil, nil },
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
