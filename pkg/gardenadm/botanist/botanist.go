// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"k8s.io/component-base/version"
	"k8s.io/utils/clock"
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
	isInitOperatingSystemConfig bool

	enableNodeAgentAuthorizer         bool
	gardenerResourceManagerServiceIPs []string
}

// Extension contains the resources needed for an extension registration.
type Extension struct {
	ControllerRegistration *gardencorev1beta1.ControllerRegistration
	ControllerDeployment   *gardencorev1.ControllerDeployment
	ControllerInstallation *gardencorev1beta1.ControllerInstallation
}

var (
	// DirFS returns a fs.FS for the files in the given directory.
	// Exposed for testing.
	DirFS = os.DirFS
	// NewFs returns an afero.Fs.
	// Exposed for testing.
	NewFs = afero.NewOsFs
)

// NewAutonomousBotanistFromManifests reads the manifests from dir and initializes a new AutonomousBotanist with them.
func NewAutonomousBotanistFromManifests(
	ctx context.Context,
	log logr.Logger,
	clientSet kubernetes.Interface,
	dir string,
	runsControlPlane bool,
) (
	*AutonomousBotanist,
	error,
) {
	resources, err := gardenadm.ReadManifests(log, DirFS(dir))
	if err != nil {
		return nil, fmt.Errorf("failed reading Kubernetes resources from config directory %s: %w", dir, err)
	}

	extensions, err := ComputeExtensions(resources, runsControlPlane)
	if err != nil {
		return nil, fmt.Errorf("failed computing extensions: %w", err)
	}

	b, err := NewAutonomousBotanist(ctx, log, clientSet, resources, extensions, runsControlPlane)
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
	resources gardenadm.Resources,
	extensions []Extension,
	runsControlPlane bool,
) (
	*AutonomousBotanist,
	error,
) {
	autonomousBotanist, err := NewAutonomousBotanistWithoutResources(log)
	if err != nil {
		return nil, fmt.Errorf("failed creating autonomous botanist: %w", err)
	}

	if err := initializeShootResource(resources.Shoot, autonomousBotanist.FS, resources.Project.Name, runsControlPlane); err != nil {
		return nil, fmt.Errorf("failed initializing shoot resource: %w", err)
	}

	initializeSeedResource(resources.Seed, resources.Shoot.Name, runsControlPlane)

	gardenClient := newFakeGardenClient()
	if err := initializeFakeGardenResources(ctx, gardenClient, resources, extensions); err != nil {
		return nil, fmt.Errorf("failed initializing resources in fake garden client: %w", err)
	}

	autonomousBotanist.Botanist, err = newBotanist(ctx, log, clientSet, gardenClient, resources, runsControlPlane)
	if err != nil {
		return nil, fmt.Errorf("failed creating botanist: %w", err)
	}

	autonomousBotanist.Extensions = extensions

	return autonomousBotanist, nil
}

// NewAutonomousBotanistWithoutResources creates a new AutonomousBotanist without instantiating a Botanist struct.
func NewAutonomousBotanistWithoutResources(log logr.Logger) (*AutonomousBotanist, error) {
	hostName, err := nodeagent.GetHostName()
	if err != nil {
		return nil, fmt.Errorf("failed fetching hostname: %w", err)
	}

	return &AutonomousBotanist{
		Botanist: &botanistpkg.Botanist{Operation: newOperation(log, newFakeGardenClient(), newFakeSeedClientSet(""))},

		HostName: hostName,
		DBus:     dbus.New(log),
		FS:       afero.Afero{Fs: NewFs()},
	}, nil
}

func newOperation(log logr.Logger, gardenClient client.Client, clientSet kubernetes.Interface) *operation.Operation {
	return &operation.Operation{
		Logger:         log,
		Clock:          clock.RealClock{},
		GardenClient:   gardenClient,
		SeedClientSet:  clientSet,
		ShootClientSet: clientSet,
	}
}

func newBotanist(
	ctx context.Context,
	log logr.Logger,
	clientSet kubernetes.Interface,
	gardenClient client.Client,
	resources gardenadm.Resources,
	runsControlPlane bool,
) (
	*botanistpkg.Botanist,
	error,
) {
	gardenObj, err := newGardenObject(ctx, resources.Project)
	if err != nil {
		return nil, fmt.Errorf("failed creating garden object: %w", err)
	}

	shootObj, err := newShootObject(ctx, gardenClient, resources, runsControlPlane)
	if err != nil {
		return nil, fmt.Errorf("failed creating shoot object: %w", err)
	}

	seedObj, err := newSeedObject(ctx, resources.Seed, shootObj)
	if err != nil {
		return nil, fmt.Errorf("failed creating seed object: %w", err)
	}

	keysAndValues := []any{"cloudProfile", resources.CloudProfile, "project", resources.Project, "shoot", resources.Shoot}
	if clientSet == nil {
		clientSet = newFakeSeedClientSet(seedObj.KubernetesVersion.String())
		log.Info("Initializing autonomous botanist with fake client set", keysAndValues...) //nolint:logcheck
	} else {
		log.Info("Initializing autonomous botanist with control plane client set", keysAndValues...) //nolint:logcheck
	}

	o := newOperation(log, gardenClient, clientSet)
	o.Garden = gardenObj
	o.Seed = seedObj
	o.Shoot = shootObj

	return botanistpkg.New(ctx, o)
}

func initializeFakeGardenResources(
	ctx context.Context,
	gardenClient client.Client,
	resources gardenadm.Resources,
	extensions []Extension,
) error {
	objects := []client.Object{resources.Seed.DeepCopy(), resources.Shoot.DeepCopy()}

	for _, extension := range extensions {
		objects = append(
			objects,
			extension.ControllerRegistration.DeepCopy(),
			extension.ControllerDeployment.DeepCopy(),
			extension.ControllerInstallation.DeepCopy(),
		)
	}

	for _, secret := range resources.Secrets {
		objects = append(objects, secret.DeepCopy())
	}

	if resources.SecretBinding != nil {
		objects = append(objects, resources.SecretBinding.DeepCopy())
	}
	if resources.CredentialsBinding != nil {
		objects = append(objects, resources.CredentialsBinding.DeepCopy())
	}

	for _, obj := range objects {
		if err := gardenClient.Create(ctx, obj); client.IgnoreAlreadyExists(err) != nil {
			return fmt.Errorf("failed creating %T %s: %w", obj, obj.GetName(), err)
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

func newSeedObject(ctx context.Context, seed *gardencorev1beta1.Seed, shootObj *shootpkg.Shoot) (*seedpkg.Seed, error) {
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
	gardenClient client.Client,
	resources gardenadm.Resources,
	runsControlPlane bool,
) (
	*shootpkg.Shoot,
	error,
) {
	b := shootpkg.
		NewBuilder().
		WithProjectName(resources.Project.Name).
		WithCloudProfileObject(resources.CloudProfile).
		WithShootObject(resources.Shoot).
		WithInternalDomain(&gardenerutils.Domain{Domain: "gardenadm.local"})

	if resources.Shoot.Spec.SecretBindingName != nil || resources.Shoot.Spec.CredentialsBindingName != nil {
		b = b.WithShootCredentialsFrom(gardenClient)
	} else {
		b = b.WithoutShootCredentials()
	}

	obj, err := b.Build(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed building shoot object: %w", err)
	}

	obj.Networks, err = shootpkg.ToNetworks(resources.Shoot, obj.IsWorkerless)
	if err != nil {
		return nil, fmt.Errorf("failed computing shoot networks: %w", err)
	}

	// In autonomous shoot clusters, kube-system is used as the control plane namespace.
	// However, when bootstrapping an autonomous shoot cluster with `gardenadm bootstrap` using a temporary local cluster,
	// we want to avoid conflicts with kube-system components of the bootstrap cluster by placing all shoot-related
	// components in another namespace. In this case, we use the technical ID as the control plane namespace, as usual.
	// TODO(timebertt): double-check if this causes problems when importing the state into the autonomous shoot cluster
	if !runsControlPlane {
		obj.ControlPlaneNamespace = resources.Shoot.Status.TechnicalID
	}

	return obj, nil
}

func newFakeGardenClient() client.Client {
	return fakeclient.
		NewClientBuilder().
		WithScheme(kubernetes.GardenScheme).
		WithStatusSubresource(
			&gardencorev1beta1.BackupBucket{},
			&gardencorev1beta1.BackupEntry{},
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

func initializeShootResource(shoot *gardencorev1beta1.Shoot, fs afero.Afero, projectName string, runsControlPlane bool) error {
	shoot.Status.TechnicalID = gardenerutils.ComputeTechnicalID(projectName, shoot)
	shoot.Status.Gardener = gardencorev1beta1.Gardener{Name: "gardenadm", Version: version.Get().GitVersion}

	if runsControlPlane {
		// This UID is used to compute the name of the BackupEntry object. Persist the generated UID on the machine in case
		// `gardenadm init` is retried/executed multiple times (otherwise, we'd always generate a new one).
		uid, err := shootUID(fs)
		if err != nil {
			return fmt.Errorf("failed fetching shoot UID: %w", err)
		}
		shoot.Status.UID = uid
	} else {
		// For `gardenadm bootstrap`, we don't need a stable UID. We generate a random one instead, because we might not be
		// able to persist the generated UID in /var/lib/gardenadm (e.g., when running `gardenadm bootstrap` on macOS).
		shoot.Status.UID = uuid.NewUUID()
	}

	return nil
}

func initializeSeedResource(seed *gardencorev1beta1.Seed, shootName string, runsControlPlane bool) {
	seed.Name = shootName
	seed.Status = gardencorev1beta1.SeedStatus{ClusterIdentity: ptr.To(shootName)}

	if runsControlPlane {
		// When running the control plane (`gardenadm init`), mark the seed as an autonomous shoot cluster.
		// Otherwise (`gardenadm bootstrap`), the bootstrap cluster should behave like a standard seed cluster.
		// If the seed is marked as an autonomous shoot cluster, extensions are configured differently, e.g., they merge the
		// shoot webhooks into the seed webhooks.
		metav1.SetMetaDataLabel(&seed.ObjectMeta, v1beta1constants.LabelAutonomousShootCluster, "true")
	}

	kubernetes.GardenScheme.Default(seed)
}

func shootUID(fs afero.Afero) (types.UID, error) {
	var (
		path                    = filepath.Join(string(filepath.Separator), "var", "lib", "gardenadm", "shoot-uid")
		permissions os.FileMode = 0600
	)

	content, err := fs.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed reading file %q: %w", path, err)
		}

		if err := fs.MkdirAll(filepath.Dir(path), permissions); err != nil {
			return "", fmt.Errorf("failed creating directory %q: %w", filepath.Dir(path), err)
		}

		content = []byte(uuid.NewUUID())
		if err := fs.WriteFile(path, content, permissions); err != nil {
			return "", fmt.Errorf("failed writing file %q: %w", path, err)
		}
	}

	return types.UID(content), nil
}
