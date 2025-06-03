// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	extensionsconfigv1alpha1 "github.com/gardener/gardener/extensions/pkg/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	// ControllerName is the name of the controller.
	ControllerName = "healthcheck"
)

// AddArgs are arguments for adding a health check controller to a controller-runtime manager.
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
	// ExtensionClass defines the extension class this extension is responsible for.
	ExtensionClass extensionsv1alpha1.ExtensionClass
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
	HealthCheckConfig extensionsconfigv1alpha1.HealthCheckConfig
	// ExtensionClass defines the extension class this extension is responsible for.
	ExtensionClass extensionsv1alpha1.ExtensionClass
}

// RegisteredExtension is a registered extensions that the HealthCheck Controller watches.
// The field extension contains any extension object
// The field healthConditionTypes contains all distinct healthCondition types (extracted from the healthCheck).
// They are used as the .type field of the Condition that the HealthCheck controller writes to the extension resource.
// The field groupVersionKind stores the GroupVersionKind of the extension resource
type RegisteredExtension struct {
	extension              extensionsv1alpha1.Object
	getExtensionObjFunc    GetExtensionObjectFunc
	healthConditionTypes   []string
	groupVersionKind       schema.GroupVersionKind
	conditionTypesToRemove sets.Set[gardencorev1beta1.ConditionType]
}

// DefaultRegistration configures the default health check NewActuator to execute the provided health checks and adds it to the provided controller-runtime manager.
// the NewActuator reconciles a single extension with a specific type and writes conditions for each distinct healthConditionTypes.
// extensionType (e.g. aws) defines the spec.type of the extension to watch
// kind defines the GroupVersionKind of the extension
// GetExtensionObjListFunc returns a client.ObjectList representation of the extension to register
// getExtensionObjFunc returns an extensionsv1alpha1.Object representation of the extension to register
// mgr is the controller runtime manager
// opts contain config for the healthcheck controller
// custom predicates allow for fine-grained control which resources to watch
// healthChecks defines the checks to execute mapped to the healthConditionTypes its contributing to (e.g checkDeployment in Seed -> ControlPlaneHealthy).
// register returns a runtime representation of the extension resource to register it with the controller-runtime
func DefaultRegistration(
	extensionType string,
	kind schema.GroupVersionKind,
	getExtensionObjListFunc GetExtensionObjectListFunc,
	getExtensionObjFunc GetExtensionObjectFunc,
	mgr manager.Manager,
	opts DefaultAddArgs,
	customPredicates []predicate.Predicate,
	healthChecks []ConditionTypeToHealthCheck,
	conditionTypesToRemove sets.Set[gardencorev1beta1.ConditionType],
) error {
	predicates := append(DefaultPredicates(), customPredicates...)
	opts.Controller.RecoverPanic = ptr.To(true)

	args := AddArgs{
		ControllerOptions:       opts.Controller,
		Predicates:              predicates,
		Type:                    extensionType,
		ExtensionClass:          opts.ExtensionClass,
		SyncPeriod:              opts.HealthCheckConfig.SyncPeriod,
		GetExtensionObjListFunc: getExtensionObjListFunc,
	}

	if err := args.RegisterExtension(getExtensionObjFunc, getHealthCheckTypes(healthChecks), kind, conditionTypesToRemove); err != nil {
		return err
	}

	var shootRestOptions extensionsconfigv1alpha1.RESTOptions
	if opts.HealthCheckConfig.ShootRESTOptions != nil {
		shootRestOptions = *opts.HealthCheckConfig.ShootRESTOptions
	}

	healthCheckActuator := NewActuator(mgr, args.Type, args.GetExtensionGroupVersionKind().Kind, getExtensionObjFunc, healthChecks, shootRestOptions)
	return Register(mgr, args, healthCheckActuator)
}

// RegisterExtension registered a resource and its corresponding healthCheckTypes.
// throws and error if the extensionResources is not a extensionsv1alpha1.Object
// The controller writes the healthCheckTypes as a condition.type into the extension resource.
// To contribute to the Shoot's health, the Gardener checks each extension for a Health Condition Type of SystemComponentsHealthy, EveryNodeReady, ControlPlaneHealthy.
// However, extensions are free to choose any healthCheckType
func (a *AddArgs) RegisterExtension(
	getExtensionObjFunc GetExtensionObjectFunc,
	conditionTypes []string,
	kind schema.GroupVersionKind,
	conditionTypesToRemove sets.Set[gardencorev1beta1.ConditionType],
) error {
	acc, err := extensions.Accessor(getExtensionObjFunc())
	if err != nil {
		return err
	}

	a.registeredExtension = &RegisteredExtension{
		extension:              acc,
		healthConditionTypes:   conditionTypes,
		groupVersionKind:       kind,
		getExtensionObjFunc:    getExtensionObjFunc,
		conditionTypesToRemove: conditionTypesToRemove,
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
	return add(mgr, args, actuator)
}

func add(mgr manager.Manager, args AddArgs, actuator HealthCheckActuator) error {
	// generate random string to create unique manager name, in case multiple managers register the same extension resource
	str, err := utils.GenerateRandomString(10)
	if err != nil {
		return err
	}

	controllerName := fmt.Sprintf("%s-%s-%s-%s-%s", ControllerName, args.registeredExtension.groupVersionKind.Kind, args.registeredExtension.groupVersionKind.Group, args.registeredExtension.groupVersionKind.Version, str)

	// add type predicate to only watch registered resource (e.g. ControlPlane) with a certain type (e.g. aws)
	predicates := predicateutils.AddTypeAndClassPredicates(args.Predicates, args.ExtensionClass, args.Type)

	log.Log.Info("Registering health check controller", "kind", args.registeredExtension.groupVersionKind.Kind, "type", args.Type, "conditionTypes", args.registeredExtension.healthConditionTypes, "syncPeriod", args.SyncPeriod.Duration.String())

	return builder.
		ControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(args.ControllerOptions).
		Watches(
			args.registeredExtension.getExtensionObjFunc(),
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicates...),
		).
		// watch Cluster of Shoot provider type (e.g. aws)
		// this is to be notified when the Shoot is being hibernated (stop health checks) and wakes up (start health checks again)
		Watches(
			&extensionsv1alpha1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(mapper.ClusterToObjectMapper(mgr.GetClient(), args.GetExtensionObjListFunc, predicates)),
		).
		Complete(NewReconciler(mgr, actuator, *args.registeredExtension, args.SyncPeriod))
}

func getHealthCheckTypes(healthChecks []ConditionTypeToHealthCheck) []string {
	types := sets.New[string]()
	for _, healthCheck := range healthChecks {
		types.Insert(healthCheck.ConditionType)
	}
	return types.UnsortedList()
}
