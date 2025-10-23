// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator

import (
	"context"
	"io"
	"reflect"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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
}

// New creates a new MutateShoot admission plugin.
func New() (*MutateShoot, error) {
	return &MutateShoot{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}, nil
}

var _ admission.MutationInterface = (*MutateShoot)(nil)

// Admit mutates the Shoot.
func (m *MutateShoot) Admit(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
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

	mutationContext := &mutationContext{
		shoot:    shoot,
		oldShoot: oldShoot,
	}

	if a.GetOperation() == admission.Create {
		addCreatedByAnnotation(shoot, a.GetUserInfo().GetName())
	}

	mutationContext.addMetadataAnnotations(a)

	return nil
}

type mutationContext struct {
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
