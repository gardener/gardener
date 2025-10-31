// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
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
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/extensions/bastion"
	"github.com/gardener/gardener/pkg/gardenadm"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	gardenpkg "github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	"github.com/gardener/gardener/pkg/nodeagent"
	"github.com/gardener/gardener/pkg/nodeagent/dbus"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	sshutils "github.com/gardener/gardener/pkg/utils/ssh"
)

// GardenadmBaseDir is the directory that gardenadm works with for storing information, transferring manifests, etc.
// NB: We don't use filepath.Join here, because we explicitly need Linux path separators for the target machine,
// even when running `gardenadm bootstrap` on Windows.
const GardenadmBaseDir = "/var/lib/gardenadm"

// GardenadmBotanist is a struct which has methods that perform operations for a self-hosted shoot cluster.
type GardenadmBotanist struct {
	*botanistpkg.Botanist

	HostName   string
	DBus       dbus.DBus
	FS         afero.Afero
	Extensions []Extension
	Resources  gardenadm.Resources

	// Bastion is only set for `gardenadm bootstrap`.
	Bastion *bastion.Bastion

	operatingSystemConfigSecret       *corev1.Secret
	gardenerResourceManagerServiceIPs []string
	staticPodNameToHash               map[string]string
	useEtcdManagedByDruid             bool

	// controlPlaneMachines is set by ListControlPlaneMachines during `gardenadm bootstrap`.
	controlPlaneMachines []machinev1alpha1.Machine
	// sshConnection is the SSH connection to the first control plane machine. It is set by ConnectToControlPlaneMachine
	// during `gardenadm bootstrap`.
	sshConnection *sshutils.Connection
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

// NewGardenadmBotanistFromManifests reads the manifests from dir and initializes a new GardenadmBotanist with them.
func NewGardenadmBotanistFromManifests(
	ctx context.Context,
	log logr.Logger,
	clientSet kubernetes.Interface,
	dir string,
	runsControlPlane bool,
) (
	*GardenadmBotanist,
	error,
) {
	resources, err := gardenadm.ReadManifests(log, DirFS(dir))
	if err != nil {
		return nil, fmt.Errorf("failed reading Kubernetes resources from config directory %s: %w", dir, err)
	}

	extensions, err := ComputeExtensions(resources, runsControlPlane, v1beta1helper.HasManagedInfrastructure(resources.Shoot))
	if err != nil {
		return nil, fmt.Errorf("failed computing extensions: %w", err)
	}

	b, err := NewGardenadmBotanist(ctx, log, clientSet, resources, extensions, runsControlPlane)
	if err != nil {
		return nil, fmt.Errorf("failed constructing botanist: %w", err)
	}

	return b, nil
}

// NewGardenadmBotanist creates a new botanist.GardenadmBotanist instance for the gardenadm command execution.
func NewGardenadmBotanist(
	ctx context.Context,
	log logr.Logger,
	clientSet kubernetes.Interface,
	resources gardenadm.Resources,
	extensions []Extension,
	runsControlPlane bool,
) (
	*GardenadmBotanist,
	error,
) {
	gardenadmBotanist, err := NewGardenadmBotanistWithoutResources(log)
	if err != nil {
		return nil, fmt.Errorf("failed creating gardenadm botanist: %w", err)
	}

	if err := initializeShootResource(resources, gardenadmBotanist.FS, runsControlPlane); err != nil {
		return nil, fmt.Errorf("failed initializing shoot resource: %w", err)
	}

	initializeSeedResource(resources, runsControlPlane)

	gardenClient := newFakeGardenClient()
	if err := initializeFakeGardenResources(ctx, gardenClient, resources, extensions); err != nil {
		return nil, fmt.Errorf("failed initializing resources in fake garden client: %w", err)
	}

	gardenadmBotanist.Botanist, err = newBotanist(ctx, log, clientSet, gardenClient, resources, runsControlPlane)
	if err != nil {
		return nil, fmt.Errorf("failed creating botanist: %w", err)
	}

	if !gardenadmBotanist.Shoot.RunsControlPlane() {
		gardenadmBotanist.Bastion = gardenadmBotanist.DefaultBastion()

		// For `gardenadm bootstrap`, we don't initialize the control plane machines with a "full OSC".
		// Instead, we provide a small alternative OSC, that only fetches the `gardenadm` binary from the registry.
		gardenadmBotanist.Shoot.Components.Extensions.OperatingSystemConfig, err = gardenadmBotanist.ControlPlaneBootstrapOperatingSystemConfig()
		if err != nil {
			return nil, err
		}
	}

	gardenadmBotanist.Resources = resources
	gardenadmBotanist.Extensions = extensions

	return gardenadmBotanist, nil
}

// NewGardenadmBotanistWithoutResources creates a new GardenadmBotanist without instantiating a Botanist struct.
func NewGardenadmBotanistWithoutResources(log logr.Logger) (*GardenadmBotanist, error) {
	hostName, err := nodeagent.GetHostName()
	if err != nil {
		return nil, fmt.Errorf("failed fetching hostname: %w", err)
	}

	return &GardenadmBotanist{
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
		log.Info("Initializing gardenadm botanist with fake client set", keysAndValues...) //nolint:logcheck
	} else {
		log.Info("Initializing gardenadm botanist with control plane client set", keysAndValues...) //nolint:logcheck
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

	for _, configMap := range resources.ConfigMaps {
		objects = append(objects, configMap.DeepCopy())
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
	if resources.ShootState != nil {
		objects = append(objects, resources.ShootState.DeepCopy())
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
		WithShootObject(resources.Shoot)

	if resources.Shoot.Spec.SecretBindingName != nil || resources.Shoot.Spec.CredentialsBindingName != nil {
		b = b.WithShootCredentialsFrom(gardenClient)
	} else {
		b = b.WithoutShootCredentials()
	}

	obj, err := b.Build(ctx, gardenClient)
	if err != nil {
		return nil, fmt.Errorf("failed building shoot object: %w", err)
	}

	obj.Networks, err = shootpkg.ToNetworks(resources.Shoot, obj.IsWorkerless)
	if err != nil {
		return nil, fmt.Errorf("failed computing shoot networks: %w", err)
	}

	// In self-hosted shoot clusters, kube-system is used as the control plane namespace.
	// However, when bootstrapping a self-hosted shoot cluster with `gardenadm bootstrap` using a temporary local cluster,
	// we want to avoid conflicts with kube-system components of the bootstrap cluster by placing all shoot-related
	// components in another namespace. In this case, we use the technical ID as the control plane namespace, as usual.
	// TODO(timebertt): double-check if this causes problems when importing the state into the self-hosted shoot cluster
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

func initializeShootResource(resources gardenadm.Resources, fs afero.Afero, runsControlPlane bool) error {
	shoot := resources.Shoot
	shoot.Status.TechnicalID = gardenerutils.ComputeTechnicalID(resources.Project.Name, shoot)
	shoot.Status.Gardener = gardencorev1beta1.Gardener{Name: "gardenadm", Version: version.Get().GitVersion}

	if runsControlPlane {
		// This UID is used to compute the name of the BackupEntry object. Persist the generated UID on the machine in case
		// `gardenadm init` is retried/executed multiple times (otherwise, we'd always generate a new one).
		uid, err := shootUID(fs)
		if err != nil {
			return fmt.Errorf("failed fetching shoot UID: %w", err)
		}
		shoot.Status.UID = uid

		if v1beta1helper.HasManagedInfrastructure(resources.Shoot) {
			// When running `gardenadm init` for a shoot with managed infrastructure, we need to restore state (secrets,
			// extensions, etc.) from the ShootState exported by `gardenadm bootstrap`.
			if resources.ShootState == nil {
				return fmt.Errorf("shoot has managed infrastructure, but ShootState is missing " +
					"(the ShootState is usually exported by `gardenadm bootstrap` and read by `gardenadm init`): " +
					"you should either use `gardenadm bootstrap` to create the self-hosted shoot cluster with managed infrastructure or " +
					"remove the `Shoot.spec.{secret,credentials}BindingName` field to mark the shoot as having unmanaged infrastructure")
			}

			// Instruct the botanist and shoot package to read the ShootState and restore the state of extensions, secrets, etc.
			shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
				Type: gardencorev1beta1.LastOperationTypeRestore,
			}
		}
	} else {
		// For `gardenadm bootstrap`, we don't need a stable UID. We generate a random one instead, because we might not be
		// able to persist the generated UID in /var/lib/gardenadm (e.g., when running `gardenadm bootstrap` on macOS).
		shoot.Status.UID = uuid.NewUUID()
	}

	return nil
}

func initializeSeedResource(resources gardenadm.Resources, runsControlPlane bool) {
	seed := resources.Seed
	seed.Name = resources.Shoot.Name
	seed.Status = gardencorev1beta1.SeedStatus{ClusterIdentity: ptr.To(resources.Shoot.Name)}

	if runsControlPlane {
		// When running the control plane (`gardenadm init`), mark the seed as a self-hosted shoot cluster.
		// Otherwise (`gardenadm bootstrap`), the bootstrap cluster should behave like a standard seed cluster.
		// If the seed is marked as a self-hosted shoot cluster, extensions are configured differently, e.g., they merge the
		// shoot webhooks into the seed webhooks.
		metav1.SetMetaDataLabel(&seed.ObjectMeta, v1beta1constants.LabelSelfHostedShootCluster, "true")
	}

	kubernetes.GardenScheme.Default(seed)
}

func shootUID(fs afero.Afero) (types.UID, error) {
	var (
		path                    = filepath.Join(string(filepath.Separator), GardenadmBaseDir, "shoot-uid")
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
