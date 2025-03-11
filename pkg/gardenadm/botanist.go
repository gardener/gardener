// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenadm

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
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
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// NewBotanist creates a new botanist.Botanist instance for the gardenadm commands execution.
func NewBotanist(
	ctx context.Context,
	log logr.Logger,
	project *gardencorev1beta1.Project,
	cloudProfile *gardencorev1beta1.CloudProfile,
	shoot *gardencorev1beta1.Shoot,
) (
	*botanistpkg.Botanist,
	error,
) {
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

	log.Info("Initializing botanist",
		"cloudProfile", cloudProfile,
		"project", project,
		"shoot", shoot)

	return botanistpkg.New(ctx, &operation.Operation{
		Logger:        log,
		GardenClient:  newFakeGardenClient(),
		SeedClientSet: newFakeSeedClientSet(seedObj.KubernetesVersion.String()),
		Garden:        gardenObj,
		Seed:          seedObj,
		Shoot:         shootObj,
	})
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
