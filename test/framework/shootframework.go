// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/test/utils/access"
)

var shootCfg *ShootConfig

// ShootConfig is the configuration for a shoot framework
type ShootConfig struct {
	GardenerConfig *GardenerConfig
	ShootName      string
	Fenced         bool
	SeedScheme     *runtime.Scheme

	CreateTestNamespace         bool
	DisableTestNamespaceCleanup bool
	SkipSeedInitialization      bool
}

// ShootFramework represents the shoot test framework that includes
// test functions that can be executed on a specific shoot
type ShootFramework struct {
	*GardenerFramework
	TestDescription
	Config *ShootConfig

	SeedClient  kubernetes.Interface
	ShootClient kubernetes.Interface

	Seed         *gardencorev1beta1.Seed
	CloudProfile *gardencorev1beta1.CloudProfile
	Shoot        *gardencorev1beta1.Shoot
	Project      *gardencorev1beta1.Project

	Namespace string
}

// NewShootFramework creates a new simple Shoot framework
func NewShootFramework(cfg *ShootConfig) *ShootFramework {
	f := &ShootFramework{
		GardenerFramework: newGardenerFrameworkFromConfig(nil),
		TestDescription:   NewTestDescription("SHOOT"),
		Config:            cfg,
	}

	CBeforeEach(func(ctx context.Context) {
		f.CommonFramework.BeforeEach()
		f.GardenerFramework.BeforeEach()
		f.BeforeEach(ctx)
	}, 8*time.Minute)
	CAfterEach(f.AfterEach, 10*time.Minute)
	return f
}

// BeforeEach should be called in ginkgo's BeforeEach.
// It sets up the shoot framework.
func (f *ShootFramework) BeforeEach(ctx context.Context) {
	f.Config = mergeShootConfig(f.Config, shootCfg)
	validateShootConfig(f.Config)
	err := f.AddShoot(ctx, f.Config.ShootName, f.ProjectNamespace)
	ExpectNoError(err)

	if f.Config.CreateTestNamespace {
		_, err := f.CreateNewNamespace(ctx)
		ExpectNoError(err)
	}
}

// AfterEach should be called in ginkgo's AfterEach.
// Cleans up resources and dumps the shoot state if the test failed
func (f *ShootFramework) AfterEach(ctx context.Context) {
	if ginkgo.CurrentSpecReport().Failed() {
		f.DumpState(ctx)
	}
	if !f.Config.DisableTestNamespaceCleanup && f.Namespace != "" {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: f.Namespace},
		}
		f.Namespace = ""
		err := f.ShootClient.Client().Delete(ctx, ns)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				ExpectNoError(err)
			}
		}
		err = f.WaitUntilNamespaceIsDeleted(ctx, f.ShootClient, ns.Name)
		if err != nil {
			timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
			defer cancel()

			err2 := f.dumpNamespaceResource(timeoutCtx, f.Logger, f.ShootClient, ns.Name)
			ExpectNoError(err2)
			err2 = f.DumpDefaultResourcesInNamespace(timeoutCtx, f.ShootClient, ns.Name)
			ExpectNoError(err2)
		}
		ExpectNoError(err)
		ginkgo.By(fmt.Sprintf("deleted test namespace %s", ns.Name))
	}
}

// CreateNewNamespace creates a new namespace with a generated name prefixed with "gardener-e2e-".
// The created namespace is automatically cleaned up when the test is finished.
func (f *ShootFramework) CreateNewNamespace(ctx context.Context) (string, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "gardener-e2e-",
		},
	}
	if err := f.ShootClient.Client().Create(ctx, ns); err != nil {
		return "", err
	}

	f.Namespace = ns.GetName()
	return ns.GetName(), nil
}

// AddShoot sets the shoot and its seed for the GardenerOperation.
func (f *ShootFramework) AddShoot(ctx context.Context, shootName, shootNamespace string) error {
	if f.GardenClient == nil {
		return errors.New("no gardener client is defined")
	}

	var (
		shoot = &gardencorev1beta1.Shoot{}
		err   error
	)

	if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: shootNamespace, Name: shootName}, shoot); err != nil {
		return fmt.Errorf("could not get shoot: %w", err)
	}

	f.CloudProfile, err = f.GardenerFramework.GetCloudProfile(ctx, shoot.Spec.CloudProfile, shoot.Namespace, shoot.Spec.CloudProfileName)
	if err != nil {
		return fmt.Errorf("unable to get cloudprofile: %w", err)
	}

	f.Project, err = f.GetShootProject(ctx, shootNamespace)
	if err != nil {
		return err
	}

	// seed could be temporarily offline so no specified seed is a valid state
	if shoot.Spec.SeedName != nil && !f.Config.SkipSeedInitialization {
		f.Seed, f.SeedClient, err = f.GetSeed(ctx, *shoot.Spec.SeedName)
		if err != nil {
			return err
		}
	}

	f.Shoot = shoot

	if f.Shoot.Spec.Hibernation != nil && f.Shoot.Spec.Hibernation.Enabled != nil && *f.Shoot.Spec.Hibernation.Enabled {
		return nil
	}

	if !f.GardenerFramework.Config.SkipAccessingShoot {
		var shootClient kubernetes.Interface
		if err := retry.UntilTimeout(ctx, k8sClientInitPollInterval, k8sClientInitTimeout, func(ctx context.Context) (bool, error) {
			shootClient, err = access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, f.Shoot)
			if err != nil {
				return retry.MinorError(fmt.Errorf("could not construct Shoot client: %w", err))
			}
			return retry.Ok()
		}); err != nil {
			return err
		}

		f.ShootClient = shootClient
	}

	return nil
}

func validateShootConfig(cfg *ShootConfig) {
	if cfg == nil {
		ginkgo.Fail("no shoot framework configuration provided")
		return
	}
	if !StringSet(cfg.ShootName) {
		ginkgo.Fail("You should specify a shootName to test against")
	}
}

func mergeShootConfig(base, overwrite *ShootConfig) *ShootConfig {
	if base == nil {
		return overwrite
	}
	if overwrite == nil {
		return base
	}

	if overwrite.GardenerConfig != nil {
		base.GardenerConfig = overwrite.GardenerConfig
	}
	if StringSet(overwrite.ShootName) {
		base.ShootName = overwrite.ShootName
	}
	if overwrite.CreateTestNamespace {
		base.CreateTestNamespace = overwrite.CreateTestNamespace
	}
	if overwrite.DisableTestNamespaceCleanup {
		base.DisableTestNamespaceCleanup = overwrite.DisableTestNamespaceCleanup
	}

	return base
}

// RegisterShootFrameworkFlags adds all flags that are needed to configure a shoot framework to the provided flagset.
func RegisterShootFrameworkFlags() *ShootConfig {
	_ = RegisterGardenerFrameworkFlags()

	newCfg := &ShootConfig{}

	flag.StringVar(&newCfg.ShootName, "shoot-name", "", "name of the shoot")
	flag.BoolVar(&newCfg.Fenced, "fenced", false,
		"indicates if the shoot is running in a fenced environment which means that the shoot can only be reached from the gardenlet")

	shootCfg = newCfg
	return shootCfg
}

// HibernateShoot hibernates the shoot of the framework
func (f *ShootFramework) HibernateShoot(ctx context.Context) error {
	return f.GardenerFramework.HibernateShoot(ctx, f.Shoot)
}

// WakeUpShoot wakes up the hibernated shoot of the framework
func (f *ShootFramework) WakeUpShoot(ctx context.Context) error {
	return f.GardenerFramework.WakeUpShoot(ctx, f.Shoot)
}

// UpdateShoot Updates a shoot from a shoot Object and waits for its reconciliation
func (f *ShootFramework) UpdateShoot(ctx context.Context, update func(shoot *gardencorev1beta1.Shoot) error) error {
	return f.GardenerFramework.UpdateShoot(ctx, f.Shoot, update)
}

// GetCloudProfile returns the cloudprofile of the shoot
func (f *ShootFramework) GetCloudProfile(ctx context.Context) (*gardencorev1beta1.CloudProfile, error) {
	if f.Shoot.Spec.CloudProfile != nil {
		cloudProfileName := f.Shoot.Spec.CloudProfile.Name
		switch f.Shoot.Spec.CloudProfile.Kind {
		case v1beta1constants.CloudProfileReferenceKindCloudProfile:
			cloudProfile := &gardencorev1beta1.CloudProfile{}
			if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Name: cloudProfileName}, cloudProfile); err != nil {
				return nil, fmt.Errorf("could not get CloudProfile '%s' in Garden cluster: %w", cloudProfileName, err)
			}
			return cloudProfile, nil
		case v1beta1constants.CloudProfileReferenceKindNamespacedCloudProfile:
			namespacedCloudProfile := &gardencorev1beta1.NamespacedCloudProfile{}
			if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Name: cloudProfileName, Namespace: f.Namespace}, namespacedCloudProfile); err != nil {
				return nil, fmt.Errorf("could not get NamespacedCloudProfile '%s' in Garden cluster: %w", cloudProfileName, err)
			}
			return &gardencorev1beta1.CloudProfile{Spec: namespacedCloudProfile.Status.CloudProfileSpec}, nil
		}
	} else if f.Shoot.Spec.CloudProfileName != nil {
		cloudProfile := &gardencorev1beta1.CloudProfile{}
		if err := f.GardenClient.Client().Get(ctx, client.ObjectKey{Name: *f.Shoot.Spec.CloudProfileName}, cloudProfile); err != nil {
			return nil, fmt.Errorf("could not get Seed's CloudProvider in Garden cluster: %w", err)
		}
		return cloudProfile, nil
	}
	return nil, errors.New("cloudprofile is required to be set in shoot spec")
}

// WaitForShootCondition waits for the shoot to contain the specified condition
func (f *ShootFramework) WaitForShootCondition(ctx context.Context, interval, timeout time.Duration, conditionType gardencorev1beta1.ConditionType, conditionStatus gardencorev1beta1.ConditionStatus) error {
	log := f.Logger.WithValues("shoot", client.ObjectKeyFromObject(f.Shoot))

	return retry.UntilTimeout(ctx, interval, timeout, func(ctx context.Context) (done bool, err error) {
		shoot := &gardencorev1beta1.Shoot{}
		err = f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: f.Shoot.Namespace, Name: f.Shoot.Name}, shoot)
		if err != nil {
			log.Error(err, "Error while waiting for shoot to have expected condition")
			return retry.MinorError(err)
		}

		cond := helper.GetCondition(shoot.Status.Conditions, conditionType)
		if cond != nil && cond.Status == conditionStatus {
			return retry.Ok()
		}

		log = log.WithValues("expectedConditionType", conditionType, "expectedConditionStatus", conditionStatus)

		if cond == nil {
			log.Info("Waiting for shoot to have expected condition status, currently the condition is not present")
			return retry.MinorError(fmt.Errorf("shoot %q does not yet have expected condition status", shoot.Name))
		}

		log.Info("Waiting for shoot to have expected condition status", "currentConditionStatus", cond.Status)
		return retry.MinorError(fmt.Errorf("shoot %q does not yet have expected condition", shoot.Name))
	})
}

// IsAPIServerRunning checks, if the Shoot's API server deployment is present, not yet deleted and has at least one
// available replica.
func (f *ShootFramework) IsAPIServerRunning(ctx context.Context) (bool, error) {
	deployment := &appsv1.Deployment{}
	if err := f.SeedClient.Client().Get(ctx, client.ObjectKey{Namespace: f.ShootSeedNamespace(), Name: v1beta1constants.DeploymentNameKubeAPIServer}, deployment); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	if deployment.GetDeletionTimestamp() != nil {
		return false, nil
	}

	return deployment.Status.AvailableReplicas > 0, nil
}
