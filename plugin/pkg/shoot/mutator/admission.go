// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"slices"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	plugin "github.com/gardener/gardener/plugin/pkg"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameShootMutator, func(_ io.Reader) (admission.Interface, error) {
		return New()
	})
}

// MutateShoot is an implementation of admission.Interface.
type MutateShoot struct {
	*admission.Handler

	seedLister gardencorev1beta1listers.SeedLister
	readyFunc  admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsCoreInformerFactory(&MutateShoot{})

	readyFuncs []admission.ReadyFunc
)

// New creates a new MutateShoot admission plugin.
func New() (*MutateShoot, error) {
	return &MutateShoot{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (v *MutateShoot) AssignReadyFunc(f admission.ReadyFunc) {
	v.readyFunc = f
	v.SetReadyFunc(f)
}

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (m *MutateShoot) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	seedInformer := f.Core().V1beta1().Seeds()
	m.seedLister = seedInformer.Lister()

	readyFuncs = append(
		readyFuncs,
		seedInformer.Informer().HasSynced,
	)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (m *MutateShoot) ValidateInitialization() error {
	if m.seedLister == nil {
		return errors.New("missing seed lister")
	}
	return nil
}

var _ admission.MutationInterface = (*MutateShoot)(nil)

// Admit mutates the Shoot.
func (m *MutateShoot) Admit(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	// Wait until the caches have been synced
	if m.readyFunc == nil {
		m.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
	if !m.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	// Ignore all kinds other than Shoot
	if a.GetKind().GroupKind() != core.Kind("Shoot") {
		return nil
	}

	var (
		shoot    *core.Shoot
		oldShoot = &core.Shoot{}
	)

	shoot, ok := a.GetObject().(*core.Shoot)
	if !ok {
		return apierrors.NewBadRequest("could not convert object to Shoot")
	}

	if a.GetOperation() == admission.Update {
		oldShoot, ok = a.GetOldObject().(*core.Shoot)
		if !ok {
			return apierrors.NewBadRequest("could not convert old object to Shoot")
		}
	}

	var seed *gardencorev1beta1.Seed
	if shoot.Spec.SeedName != nil {
		var err error
		seed, err = m.seedLister.Get(*shoot.Spec.SeedName)
		if err != nil {
			return apierrors.NewInternalError(fmt.Errorf("could not find referenced seed %q: %w", *shoot.Spec.SeedName, err))
		}
	}

	mutationContext := &mutationContext{
		seed:     seed,
		shoot:    shoot,
		oldShoot: oldShoot,
	}

	if a.GetOperation() == admission.Create {
		addCreatedByAnnotation(shoot, a.GetUserInfo().GetName())
	}

	mutationContext.addMetadataAnnotations(a)
	mutationContext.defaultShootNetworks(helper.IsWorkerless(shoot))

	return nil
}

type mutationContext struct {
	seed     *gardencorev1beta1.Seed
	shoot    *core.Shoot
	oldShoot *core.Shoot
}

func addCreatedByAnnotation(shoot *core.Shoot, userName string) {
	metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.GardenCreatedBy, userName)
}

func (c *mutationContext) addMetadataAnnotations(a admission.Attributes) {
	if a.GetOperation() == admission.Create {
		addInfrastructureDeploymentTask(c.shoot)
		addDNSRecordDeploymentTasks(c.shoot)
	}

	var (
		oldIsHibernated = c.oldShoot.Spec.Hibernation != nil && c.oldShoot.Spec.Hibernation.Enabled != nil && *c.oldShoot.Spec.Hibernation.Enabled
		newIsHibernated = c.shoot.Spec.Hibernation != nil && c.shoot.Spec.Hibernation.Enabled != nil && *c.shoot.Spec.Hibernation.Enabled
	)

	if !newIsHibernated && oldIsHibernated {
		addInfrastructureDeploymentTask(c.shoot)
		addDNSRecordDeploymentTasks(c.shoot)
	}

	if !reflect.DeepEqual(c.oldShoot.Spec.Provider.InfrastructureConfig, c.shoot.Spec.Provider.InfrastructureConfig) ||
		c.oldShoot.Spec.Networking != nil && c.oldShoot.Spec.Networking.IPFamilies != nil && !reflect.DeepEqual(c.oldShoot.Spec.Networking.IPFamilies, c.shoot.Spec.Networking.IPFamilies) {
		addInfrastructureDeploymentTask(c.shoot)
	}

	// We rely that SSHAccess is defaulted in the shoot creation, that is why we do not check for nils for the new shoot object.
	if c.oldShoot.Spec.Provider.WorkersSettings != nil &&
		c.oldShoot.Spec.Provider.WorkersSettings.SSHAccess != nil &&
		c.oldShoot.Spec.Provider.WorkersSettings.SSHAccess.Enabled != c.shoot.Spec.Provider.WorkersSettings.SSHAccess.Enabled {
		addInfrastructureDeploymentTask(c.shoot)
	}

	if !reflect.DeepEqual(c.oldShoot.Spec.DNS, c.shoot.Spec.DNS) {
		addDNSRecordDeploymentTasks(c.shoot)
	}

	if sets.New(
		v1beta1constants.ShootOperationRotateSSHKeypair,
		v1beta1constants.OperationRotateCredentialsStart,
		v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout,
	).Has(c.shoot.Annotations[v1beta1constants.GardenerOperation]) {
		addInfrastructureDeploymentTask(c.shoot)
	}

	if c.shoot.Spec.Maintenance != nil &&
		ptr.Deref(c.shoot.Spec.Maintenance.ConfineSpecUpdateRollout, false) &&
		!apiequality.Semantic.DeepEqual(c.oldShoot.Spec, c.shoot.Spec) &&
		c.shoot.Status.LastOperation != nil &&
		c.shoot.Status.LastOperation.State == core.LastOperationStateFailed {
		metav1.SetMetaDataAnnotation(&c.shoot.ObjectMeta, v1beta1constants.FailedShootNeedsRetryOperation, "true")
	}
}

func addInfrastructureDeploymentTask(shoot *core.Shoot) {
	addDeploymentTasks(shoot, v1beta1constants.ShootTaskDeployInfrastructure)
}

func addDNSRecordDeploymentTasks(shoot *core.Shoot) {
	addDeploymentTasks(shoot,
		v1beta1constants.ShootTaskDeployDNSRecordInternal,
		v1beta1constants.ShootTaskDeployDNSRecordExternal,
		v1beta1constants.ShootTaskDeployDNSRecordIngress,
	)
}

func addDeploymentTasks(shoot *core.Shoot, tasks ...string) {
	if shoot.Annotations == nil {
		shoot.Annotations = make(map[string]string)
	}
	controllerutils.AddTasks(shoot.Annotations, tasks...)
}

func (c *mutationContext) defaultShootNetworks(workerless bool) {
	if c.seed != nil {
		if c.shoot.Spec.Networking.Pods == nil && !workerless {
			if c.seed.Spec.Networks.ShootDefaults != nil && c.seed.Spec.Networks.ShootDefaults.Pods != nil {
				if cidrMatchesIPFamily(*c.seed.Spec.Networks.ShootDefaults.Pods, c.shoot.Spec.Networking.IPFamilies) {
					c.shoot.Spec.Networking.Pods = c.seed.Spec.Networks.ShootDefaults.Pods
				}
			}
		}

		if c.shoot.Spec.Networking.Services == nil {
			if c.seed.Spec.Networks.ShootDefaults != nil && c.seed.Spec.Networks.ShootDefaults.Services != nil {
				if cidrMatchesIPFamily(*c.seed.Spec.Networks.ShootDefaults.Services, c.shoot.Spec.Networking.IPFamilies) {
					c.shoot.Spec.Networking.Services = c.seed.Spec.Networks.ShootDefaults.Services
				}
			}
		}
	}
}

func cidrMatchesIPFamily(cidr string, ipfamilies []core.IPFamily) bool {
	ip, _, _ := net.ParseCIDR(cidr)
	return ip != nil && (ip.To4() != nil && slices.Contains(ipfamilies, core.IPFamilyIPv4) || ip.To4() == nil && slices.Contains(ipfamilies, core.IPFamilyIPv6))
}
