// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"k8s.io/component-base/version"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenadm"
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

	HostName   string
	DBus       dbus.DBus
	FS         afero.Afero
	Extensions []Extension

	operatingSystemConfigSecret *corev1.Secret
}

// Extension contains the resources needed for an extension registration.
type Extension struct {
	ControllerRegistration *gardencorev1beta1.ControllerRegistration
	ControllerDeployment   *gardencorev1.ControllerDeployment
	ControllerInstallation *gardencorev1beta1.ControllerInstallation
}

// DirFS returns an fs.FS for the files in the given directory.
// Exposed for testing.
var DirFS = os.DirFS

// NewAutonomousBotanistFromManifests reads the manifests from dir and initializes a new AutonomousBotanist with them.
func NewAutonomousBotanistFromManifests(
	ctx context.Context,
	log logr.Logger,
	clientSet kubernetes.Interface,
	dir string,
	runsControlPlane bool,
) (*AutonomousBotanist, error) {
	cloudProfile, project, shoot, controllerRegistrations, controllerDeployments, err := gardenadm.ReadManifests(log, DirFS(dir))
	if err != nil {
		return nil, fmt.Errorf("failed reading Kubernetes resources from config directory %s: %w", dir, err)
	}

	extensions, err := ComputeExtensions(shoot, controllerRegistrations, controllerDeployments, runsControlPlane)
	if err != nil {
		return nil, fmt.Errorf("failed computing extensions: %w", err)
	}

	b, err := NewAutonomousBotanist(ctx, log, clientSet, project, cloudProfile, shoot, extensions, runsControlPlane)
	if err != nil {
		return nil, fmt.Errorf("failed constructing botanist: %w", err)
	}

	return b, nil
}

// NewAutonomousBotanist creates a new botanist.AutonomousBotanist instance for the gardenadm command execution.
func NewAutonomousBotanist(
	ctx context.Context,
	log logr.Logger,
	clientSet kubernetes.Interface,
	project *gardencorev1beta1.Project,
	cloudProfile *gardencorev1beta1.CloudProfile,
	shoot *gardencorev1beta1.Shoot,
	extensions []Extension,
	runsControlPlane bool,
) (
	*AutonomousBotanist,
	error,
) {
	gardenObj, err := newGardenObject(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("failed creating garden object: %w", err)
	}

	shootObj, err := newShootObject(ctx, project.Name, cloudProfile, shoot, runsControlPlane)
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

	o := newOperation(log, clientSet)
	o.Garden = gardenObj
	o.Seed = seedObj
	o.Shoot = shootObj

	b, err := botanistpkg.New(ctx, o)
	if err != nil {
		return nil, fmt.Errorf("failed creating botanist: %w", err)
	}

	autonomousBotanist, err := NewAutonomousBotanistWithoutResources(log)
	if err != nil {
		return nil, fmt.Errorf("failed creating autonomous botanist: %w", err)
	}

	autonomousBotanist.Botanist = b
	autonomousBotanist.Extensions = extensions

	if err := autonomousBotanist.initializeFakeGardenResources(ctx); err != nil {
		return nil, fmt.Errorf("failed initializing resources in fake garden client: %w", err)
	}

	return autonomousBotanist, nil
}

// NewAutonomousBotanistWithoutResources creates a new AutonomousBotanist without instantiating a Botanist struct.
func NewAutonomousBotanistWithoutResources(log logr.Logger) (*AutonomousBotanist, error) {
	hostName, err := nodeagent.GetHostName()
	if err != nil {
		return nil, fmt.Errorf("failed fetching hostname: %w", err)
	}

	return &AutonomousBotanist{
		Botanist: &botanistpkg.Botanist{Operation: newOperation(log, newFakeSeedClientSet(""))},

		HostName: hostName,
		DBus:     dbus.New(log),
		FS:       afero.Afero{Fs: afero.NewOsFs()},
	}, nil
}

func newOperation(log logr.Logger, clientSet kubernetes.Interface) *operation.Operation {
	return &operation.Operation{
		Logger:         log,
		GardenClient:   newFakeGardenClient(),
		SeedClientSet:  clientSet,
		ShootClientSet: clientSet,
	}
}

func (b *AutonomousBotanist) initializeFakeGardenResources(ctx context.Context) error {
	if err := b.GardenClient.Create(ctx, b.Seed.GetInfo().DeepCopy()); client.IgnoreAlreadyExists(err) != nil {
		return fmt.Errorf("failed creating Seed %s: %w", b.Seed.GetInfo().Name, err)
	}

	if err := b.GardenClient.Create(ctx, b.Shoot.GetInfo().DeepCopy()); client.IgnoreAlreadyExists(err) != nil {
		return fmt.Errorf("failed creating Shoot %s: %w", client.ObjectKeyFromObject(b.Shoot.GetInfo()), err)
	}

	for _, extension := range b.Extensions {
		if err := b.GardenClient.Create(ctx, extension.ControllerRegistration.DeepCopy()); client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed creating ControllerRegistration %s: %w", extension.ControllerRegistration.Name, err)
		}

		if err := b.GardenClient.Create(ctx, extension.ControllerDeployment.DeepCopy()); client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed creating ControllerDeployment %s: %w", extension.ControllerDeployment.Name, err)
		}

		if err := b.GardenClient.Create(ctx, extension.ControllerInstallation.DeepCopy()); client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed creating ControllerInstallation %s: %w", extension.ControllerInstallation.Name, err)
		}
	}

	return nil
}

func newGardenObject(ctx context.Context, project *gardencorev1beta1.Project) (*gardenpkg.Garden, error) {
	return gardenpkg.
		NewBuilder().
		WithProject(project).
		Build(ctx)
}

func newSeedObject(ctx context.Context, shootObj *shootpkg.Shoot) (*seedpkg.Seed, error) {
	seed := &gardencorev1beta1.Seed{
		ObjectMeta: metav1.ObjectMeta{
			Name:   shootObj.GetInfo().Name,
			Labels: map[string]string{v1beta1constants.LabelAutonomousShootCluster: "true"},
		},
		Status: gardencorev1beta1.SeedStatus{ClusterIdentity: ptr.To(shootObj.GetInfo().Name)},
	}
	kubernetes.GardenScheme.Default(seed)

	obj, err := seedpkg.
		NewBuilder().
		WithSeedObject(seed).
		Build(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed building seed object: %w", err)
	}

	obj.KubernetesVersion = shootObj.KubernetesVersion
	return obj, nil
}

func newShootObject(
	ctx context.Context,
	projectName string,
	cloudProfile *gardencorev1beta1.CloudProfile,
	shoot *gardencorev1beta1.Shoot,
	runsControlPlane bool,
) (*shootpkg.Shoot, error) {
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
		WithInternalDomain(&gardenerutils.Domain{Domain: "gardenadm.local"}).
		Build(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed building shoot object: %w", err)
	}

	obj.Networks, err = shootpkg.ToNetworks(shoot, obj.IsWorkerless)
	if err != nil {
		return nil, fmt.Errorf("failed computing shoot networks: %w", err)
	}

	// In autonomous shoot clusters, kube-system is used as the control plane namespace.
	// However, when bootstrapping an autonomous shoot cluster with `gardenadm bootstrap` using a temporary local cluster,
	// we want to avoid conflicts with kube-system components of the bootstrap cluster by placing all shoot-related
	// components in another namespace. In this case, we use the technical ID as the control plane namespace, as usual.
	// TODO(timebertt): double-check if this causes problems when importing the state into the autonomous shoot cluster
	if !runsControlPlane {
		obj.ControlPlaneNamespace = shoot.Status.TechnicalID
	}

	return obj, nil
}

func newFakeGardenClient() client.Client {
	return fakeclient.
		NewClientBuilder().
		WithScheme(kubernetes.GardenScheme).
		WithStatusSubresource(
			&gardencorev1beta1.ControllerInstallation{},
			&gardencorev1beta1.Shoot{},
		).
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
