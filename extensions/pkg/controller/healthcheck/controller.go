// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package healthcheck

import (
	"fmt"

	extensionsconfig "github.com/gardener/gardener/extensions/pkg/apis/config"
	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	"github.com/gardener/gardener/pkg/api/extensions"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	"github.com/gardener/gardener/pkg/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// ControllerName is the name of the controller.
	ControllerName = "healthcheck"
)

// AddArgs are arguments for adding an health check controller to a controller-runtime manager.
type AddArgs struct {
	// ControllerOptions are the controller options used for creating a controller.
	// The options.Reconciler is always overridden with a reconciler created from the
	// given actuator.
	ControllerOptions controller.Options
	// Predicates are the predicates to use.
	// If unset, GenerationChanged will be used.
	Predicates []predicate.Predicate
	// Type is the type of the resource considered for reconciliation.
	Type string
	// SyncPeriod is the duration how often the registered extension is being reconciled
	SyncPeriod metav1.Duration
	// registeredExtension is the registered extensions that the HealthCheck Controller watches and writes HealthConditions for.
	// The Gardenlet reads the conditions on the extension Resource.
	// Through this mechanism, the extension can contribute to the Shoot's HealthStatus.
	registeredExtension *RegisteredExtension
	// GetExtensionObjListFunc returns a client.ObjectList representation of the extension to register
	GetExtensionObjListFunc GetExtensionObjectListFunc
}

// DefaultAddArgs are the default Args for the health check controller.
type DefaultAddArgs struct {
	// Controller are the controller.Options.
	Controller controller.Options
	// HealthCheckConfig contains additional config for the health check controller
	HealthCheckConfig extensionsconfig.HealthCheckConfig
}

// RegisteredExtension is a registered extensions that the HealthCheck Controller watches.
// The field extension contains any extension object
// The field healthConditionTypes contains all distinct healthCondition types (extracted from the healthCheck).
// They are used as the .type field of the Condition that the HealthCheck controller writes to the extension resource.
// The field groupVersionKind stores the GroupVersionKind of the extension resource
type RegisteredExtension struct {
	extension            extensionsv1alpha1.Object
	getExtensionObjFunc  GetExtensionObjectFunc
	healthConditionTypes []string
	groupVersionKind     schema.GroupVersionKind
}

// DefaultRegistration configures the default health check NewActuator to execute the provided health checks and adds it to the provided controller-runtime manager.
// the NewActuator reconciles a single extension with a specific type and writes conditions for each distinct healthConditionTypes.
// extensionType (e.g aws) defines the spec.type of the extension to watch
// kind defines the GroupVersionKind of the extension
// GetExtensionObjListFunc returns a client.ObjectList representation of the extension to register
// getExtensionObjFunc returns a extensionsv1alpha1.Object representation of the extension to register
// mgr is the controller runtime manager
// opts contain config for the healthcheck controller
// custom predicates allow for fine-grained control which resources to watch
// healthChecks defines the checks to execute mapped to the healthConditionTypes its contributing to (e.g checkDeployment in Seed -> ControlPlaneHealthy).
// register returns a runtime representation of the extension resource to register it with the controller-runtime
func DefaultRegistration(extensionType string, kind schema.GroupVersionKind, getExtensionObjListFunc GetExtensionObjectListFunc, getExtensionObjFunc GetExtensionObjectFunc, mgr manager.Manager, opts DefaultAddArgs, customPredicates []predicate.Predicate, healthChecks []ConditionTypeToHealthCheck) error {
	predicates := append(DefaultPredicates(), customPredicates...)
	opts.Controller.RecoverPanic = true

	args := AddArgs{
		ControllerOptions:       opts.Controller,
		Predicates:              predicates,
		Type:                    extensionType,
		SyncPeriod:              opts.HealthCheckConfig.SyncPeriod,
		GetExtensionObjListFunc: getExtensionObjListFunc,
	}

	if err := args.RegisterExtension(getExtensionObjFunc, getHealthCheckTypes(healthChecks), kind); err != nil {
		return err
	}

	var shootRestOptions extensionsconfig.RESTOptions
	if opts.HealthCheckConfig.ShootRESTOptions != nil {
		shootRestOptions = *opts.HealthCheckConfig.ShootRESTOptions
	}

	healthCheckActuator := NewActuator(args.Type, args.GetExtensionGroupVersionKind().Kind, getExtensionObjFunc, healthChecks, shootRestOptions)
	return Register(mgr, args, healthCheckActuator)
}

// RegisterExtension registered a resource and its corresponding healthCheckTypes.
// throws and error if the extensionResources is not a extensionsv1alpha1.Object
// The controller writes the healthCheckTypes as a condition.type into the extension resource.
// To contribute to the Shoot's health, the Gardener checks each extension for a Health Condition Type of SystemComponentsHealthy, EveryNodeReady, ControlPlaneHealthy.
// However extensions are free to choose any healthCheckType
func (a *AddArgs) RegisterExtension(getExtensionObjFunc GetExtensionObjectFunc, conditionTypes []string, kind schema.GroupVersionKind) error {
	acc, err := extensions.Accessor(getExtensionObjFunc())
	if err != nil {
		return err
	}

	a.registeredExtension = &RegisteredExtension{
		extension:            acc,
		healthConditionTypes: conditionTypes,
		groupVersionKind:     kind,
		getExtensionObjFunc:  getExtensionObjFunc,
	}
	return nil
}

// GetExtensionGroupVersionKind returns the schema.GroupVersionKind of the registered extension of this AddArgs.
func (a *AddArgs) GetExtensionGroupVersionKind() schema.GroupVersionKind {
	return a.registeredExtension.groupVersionKind
}

// DefaultPredicates returns the default predicates.
func DefaultPredicates() []predicate.Predicate {
	return []predicate.Predicate{
		// watch: only requeue on spec change to prevent infinite loop
		// health checks are being executed every 'sync period' anyways
		predicate.GenerationChangedPredicate{},
	}
}

// Register the extension resource. Must be of type extensionsv1alpha1.Object
// Add creates a new Reconciler and adds it to the Manager.
// and Start it when the Manager is Started.
func Register(mgr manager.Manager, args AddArgs, actuator HealthCheckActuator) error {
	args.ControllerOptions.Reconciler = NewReconciler(actuator, *args.registeredExtension, args.SyncPeriod)
	args.ControllerOptions.RecoverPanic = true
	return add(mgr, args)
}

func add(mgr manager.Manager, args AddArgs) error {
	// generate random string to create unique manager name, in case multiple managers register the same extension resource
	str, err := utils.GenerateRandomString(10)
	if err != nil {
		return err
	}

	controllerName := fmt.Sprintf("%s-%s-%s-%s-%s", ControllerName, args.registeredExtension.groupVersionKind.Kind, args.registeredExtension.groupVersionKind.Group, args.registeredExtension.groupVersionKind.Version, str)
	ctrl, err := controller.New(controllerName, mgr, args.ControllerOptions)
	if err != nil {
		return err
	}

	log.Log.Info("Registered health check controller", "kind", args.registeredExtension.groupVersionKind.Kind, "type", args.Type, "conditionTypes", args.registeredExtension.healthConditionTypes, "syncPeriod", args.SyncPeriod.Duration.String())

	// add type predicate to only watch registered resource (e.g ControlPlane) with a certain type (e.g aws)
	predicates := extensionspredicate.AddTypePredicate(args.Predicates, args.Type)

	if err := ctrl.Watch(&source.Kind{Type: args.registeredExtension.getExtensionObjFunc()}, &handler.EnqueueRequestForObject{}, predicates...); err != nil {
		return err
	}

	// watch Cluster of Shoot provider type (e.g aws)
	// this is to be notified when the Shoot is being hibernated (stop health checks) and wakes up (start health checks again)
	return ctrl.Watch(
		&source.Kind{Type: &extensionsv1alpha1.Cluster{}},
		mapper.EnqueueRequestsFrom(mapper.ClusterToObjectMapper(args.GetExtensionObjListFunc, predicates), mapper.UpdateWithNew, ctrl.GetLogger()),
	)
}

func getHealthCheckTypes(healthChecks []ConditionTypeToHealthCheck) []string {
	types := sets.NewString()
	for _, healthCheck := range healthChecks {
		types.Insert(healthCheck.ConditionType)
	}
	return types.UnsortedList()
}
