// SPDX-FileCopyrightText: Contributors to the Gardener project
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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	"k8s.io/component-base/version"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/imagevector"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenadm"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
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

	HostName string
	DBus     dbus.DBus
	FS       afero.Afero

	Resources  gardenadm.Resources
	Extensions []Extension
	// Zone is the availability zone in which the new node is being added. This is used to set the
	// topology.kubernetes.io/zone label on the node resource.
	Zone *string

	operatingSystemConfigSecret *corev1.Secret

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

	gardenClient := newFakeGardenClient()
	if err := initializeFakeGardenResources(ctx, gardenClient, resources, extensions); err != nil {
		return nil, fmt.Errorf("failed initializing resources in fake garden client: %w", err)
	}

	gardenadmBotanist.Botanist, err = newBotanist(ctx, log, clientSet, gardenClient, resources, runsControlPlane)
	if err != nil {
		return nil, fmt.Errorf("failed creating botanist: %w", err)
	}

	if !gardenadmBotanist.Shoot.RunsControlPlane() {
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

	gardenletConfig := &gardenletconfigv1alpha1.GardenletConfiguration{}
	gardenletconfigv1alpha1.SetObjectDefaults_GardenletConfiguration(gardenletConfig)

	// Stub `Operation`; used only for hostname/client-set creation (e.g., `gardenadm join`).
	b := &GardenadmBotanist{
		Botanist: &botanistpkg.Botanist{Operation: &operation.Operation{
			Logger:         log,
			Clock:          clock.RealClock{},
			Config:         gardenletConfig,
			GardenClient:   newFakeGardenClient(),
			SeedClientSet:  newFakeSeedClientSet(""),
			ShootClientSet: newFakeSeedClientSet(""),
		}},

		HostName: hostName,
		DBus:     dbus.New(log),
		FS:       afero.Afero{Fs: NewFs()},
	}

	if caBundle := imagevector.ContainersCABundle(); caBundle != nil && caBundle.Inline != nil {
		b.RegistryCABundle = caBundle.Inline
	}

	return b, nil
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
	keysAndValues := []any{"cloudProfile", resources.CloudProfile, "project", resources.Project, "shoot", resources.Shoot}
	if clientSet == nil {
		clientSet = newFakeSeedClientSet(resources.Shoot.Spec.Kubernetes.Version)
		log.Info("Initializing gardenadm botanist with fake client set", keysAndValues...) //nolint:logcheck
	} else {
		log.Info("Initializing gardenadm botanist with control plane client set", keysAndValues...) //nolint:logcheck
	}

	gardenletConfig := &gardenletconfigv1alpha1.GardenletConfiguration{}
	gardenletconfigv1alpha1.SetObjectDefaults_GardenletConfiguration(gardenletConfig)

	o, err := operation.Initialize(
		ctx,
		log,
		gardenClient,
		gardenClient,
		clientSet,
		nil, // gardenadm has no ShootClientMap
		gardenletConfig,
		&gardencorev1beta1.Gardener{Name: "gardenadm", Version: version.Get().GitVersion},
		resources.Shoot.Name,
		resources.Shoot,
		resources.Project,
		resources.CloudProfile,
		nil, // no `Seed` in gardenadm
		nil, // no `ExposureClass` in gardenadm
	)
	if err != nil {
		return nil, fmt.Errorf("failed initializing operation: %w", err)
	}

	if !runsControlPlane {
		// In self-hosted shoot clusters, kube-system is used as the control plane namespace.
		// However, when bootstrapping a self-hosted shoot cluster with `gardenadm bootstrap` using a temporary local cluster,
		// we want to avoid conflicts with kube-system components of the bootstrap cluster by placing all shoot-related
		// components in another namespace. In this case, we use the technical ID as the control plane namespace, as usual.
		o.Shoot.ControlPlaneNamespace = resources.Shoot.Status.TechnicalID
	}

	return botanistpkg.New(ctx, o)
}

func initializeFakeGardenResources(
	ctx context.Context,
	gardenClient client.Client,
	resources gardenadm.Resources,
	extensions []Extension,
) error {
	objects := []client.Object{resources.Shoot.DeepCopy()}

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
		s := secret.DeepCopy()
		// The fake garden client does not run the kube-apiserver's Secret strategy, which merges stringData into data.
		// Do it here so consumers reading secret.Data (e.g. the OCI HelmRegistry reading the CA bundle) behave as they
		// would against a real garden cluster.
		if len(s.StringData) > 0 {
			if s.Data == nil {
				s.Data = make(map[string][]byte, len(s.StringData))
			}
			for k, v := range s.StringData {
				s.Data[k] = []byte(v)
			}
			s.StringData = nil
		}
		objects = append(objects, s)
	}
	for _, workloadIdentity := range resources.WorkloadIdentities {
		objects = append(objects, workloadIdentity.DeepCopy())
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
		// The Shoot UID determines the BackupBucket/BackupEntry names and thus the etcd backup location,
		// so it must stay stable across invocations.
		//
		// - `gardenadm restore`: UID comes from the Shoot status exported by `gardenadm discover existing`; preserve it.
		// - `gardenadm init`: no UID on the Shoot, so reuse the one persisted at /var/lib/gardenadm/shoot-uid,
		//   or generate and persist a new one (keeps retries of `gardenadm init` idempotent).
		if shoot.Status.UID == "" {
			uid, err := shootUID(fs)
			if err != nil {
				return fmt.Errorf("failed fetching shoot UID: %w", err)
			}
			shoot.Status.UID = uid
		}

		if v1beta1helper.HasManagedInfrastructure(resources.Shoot) && resources.ShootState == nil {
			return fmt.Errorf("shoot has managed infrastructure, but ShootState is missing " +
				"(the ShootState is usually exported by `gardenadm bootstrap` and read by `gardenadm init`): " +
				"you should either use `gardenadm bootstrap` to create the self-hosted shoot cluster with managed infrastructure or " +
				"remove the `Shoot.spec.{secret,credentials}BindingName` field to mark the shoot as having unmanaged infrastructure")
		}

		if resources.ShootState != nil {
			// Instruct the botanist and shoot package to read the ShootState and restore the state of extensions, secrets, etc.
			// For managed infrastructure, this restores the state exported by `gardenadm bootstrap`.
			// For unmanaged infrastructure on retry, this restores the bootstrap secrets persisted by `gardenadm init`.
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
