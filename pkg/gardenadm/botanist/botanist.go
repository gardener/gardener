// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"k8s.io/component-base/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	gardenpkg "github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/nodeagent"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// AutonomousBotanist is a struct which has methods that perform operations for an autonomous shoot cluster.
type AutonomousBotanist struct {
	*botanistpkg.Botanist

	HostName string
	DBus     dbus.DBus
	FS       afero.Afero

	operatingSystemConfigSecret *corev1.Secret
}

// NewAutonomousBotanist creates a new botanist.AutonomousBotanist instance for the gardenadm command execution.
func NewAutonomousBotanist(
	ctx context.Context,
	log logr.Logger,
	clientSet kubernetes.Interface,
	project *gardencorev1beta1.Project,
	cloudProfile *gardencorev1beta1.CloudProfile,
	shoot *gardencorev1beta1.Shoot,
) (
	*AutonomousBotanist,
	error,
) {
	hostName, err := nodeagent.GetHostName()
	if err != nil {
		return nil, fmt.Errorf("failed fetching hostname: %w", err)
	}

	gardenObj, err := newGardenObject(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("failed creating garden object: %w", err)
	}

	shootObj, err := newShootObject(ctx, project.Name, cloudProfile, shoot)
	if err != nil {
		return nil, fmt.Errorf("failed creating shoot object: %w", err)
	}

	seedObj, err := newSeedObject(ctx, shootObj)
	if err != nil {
		return nil, fmt.Errorf("failed creating seed object: %w", err)
	}

	keysAndValues := []any{"cloudProfile", cloudProfile, "project", project, "shoot", shoot}
	if clientSet == nil {
		clientSet = newFakeSeedClientSet(seedObj.KubernetesVersion.String())
		log.Info("Initializing autonomous botanist with fake client set", keysAndValues...) //nolint:logcheck
	} else {
		log.Info("Initializing autonomous botanist with control plane client set", keysAndValues...) //nolint:logcheck
	}

	b, err := botanistpkg.New(ctx, &operation.Operation{
		Logger:         log,
		GardenClient:   newFakeGardenClient(),
		SeedClientSet:  clientSet,
		ShootClientSet: clientSet,
		Garden:         gardenObj,
		Seed:           seedObj,
		Shoot:          shootObj,
	})
	if err != nil {
		return nil, fmt.Errorf("failed creating botanist: %w", err)
	}

	return &AutonomousBotanist{
		Botanist: b,

		HostName: hostName,
		DBus:     dbus.New(log),
		FS:       afero.Afero{Fs: afero.NewOsFs()},
	}, nil
}

func newGardenObject(ctx context.Context, project *gardencorev1beta1.Project) (*gardenpkg.Garden, error) {
	return gardenpkg.
		NewBuilder().
		WithProject(project).
		Build(ctx)
}

func newSeedObject(ctx context.Context, shootObj *shootpkg.Shoot) (*seedpkg.Seed, error) {
	obj, err := seedpkg.
		NewBuilder().
		WithSeedObject(&gardencorev1beta1.Seed{}).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed building seed object: %w", err)
	}

	obj.KubernetesVersion = shootObj.KubernetesVersion
	return obj, nil
}

func newShootObject(ctx context.Context, projectName string, cloudProfile *gardencorev1beta1.CloudProfile, shoot *gardencorev1beta1.Shoot) (*shootpkg.Shoot, error) {
	shoot.Status.TechnicalID = gardenerutils.ComputeTechnicalID(projectName, shoot)
	shoot.Status.Gardener = gardencorev1beta1.Gardener{Name: "gardenadm", Version: version.Get().GitVersion}
	// TODO(rfranzke): This UID is used to compute the name of the BackupEntry object. Consider persisting this random
	//  UID on the machine in case `gardenadm init` is retried/executed multiple times (otherwise, we'd always generate
	//  a new one).
	shoot.Status.UID = uuid.NewUUID()

	obj, err := shootpkg.
		NewBuilder().
		WithProjectName(projectName).
		WithCloudProfileObject(cloudProfile).
		WithShootObject(shoot).
		Build(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed building shoot object: %w", err)
	}

	obj.Networks, err = shootpkg.ToNetworks(shoot, obj.IsWorkerless)
	if err != nil {
		return nil, fmt.Errorf("failed computing shoot networks: %w", err)
	}

	obj.ControlPlaneNamespace = metav1.NamespaceSystem
	return obj, nil
}

func newFakeGardenClient() client.Client {
	return fakeclient.
		NewClientBuilder().
		WithScheme(kubernetes.GardenScheme).
		Build()
}

func newFakeSeedClientSet(kubernetesVersion string) kubernetes.Interface {
	return fakekubernetes.
		NewClientSetBuilder().
		WithClient(fakeclient.
			NewClientBuilder().
			WithScheme(kubernetes.SeedScheme).
			Build(),
		).
		WithRESTConfig(&rest.Config{}).
		WithVersion(kubernetesVersion).
		Build()
}
