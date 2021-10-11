package rootcapublisher

import (
	"fmt"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/cluster"

	"github.com/spf13/pflag"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// ControllerName is the name of the root ca controller.
const ControllerName = "root-ca-controller"

// defaultControllerConfig is the default config for the controller.
var defaultControllerConfig ControllerConfig

// ControllerConfig is the completed configuration for the controller.
type ControllerOptions struct {
	maxConcurrentWorkers int
	rootCAPath           string
}

// ControllerConfig is the completed configuration for the controller.
type ControllerConfig struct {
	MaxConcurrentWorkers int
	RootCAPath           string
	TargetCluster        cluster.Cluster
}

func AddToManagerWithOptions(mgr manager.Manager, conf ControllerConfig) error {
	if conf.RootCAPath == "" {

		return nil
	}

	rootCA, err := os.ReadFile(conf.RootCAPath)
	if err != nil {
		return fmt.Errorf("file for root ca could not be read: %w", err)
	}

	rootcaController, err := controller.New(ControllerName, mgr, controller.Options{
		MaxConcurrentReconciles: conf.MaxConcurrentWorkers,
		Reconciler: &reconciler{
			rootCA:       string(rootCA),
			targetClient: conf.TargetClientConfig.Client,
		},
	})
	if err != nil {
		return fmt.Errorf("unable to set up root ca controller: %w", err)
	}

	if err := rootcaController.Watch(
		&source.Kind{Type: &corev1.Namespace{}},
		&handler.EnqueueRequestForObject{},
		predicate.Funcs{
			CreateFunc:  func(e event.CreateEvent) bool { return true },
			UpdateFunc:  func(e event.UpdateEvent) bool { return isActive(e.ObjectNew) },
			DeleteFunc:  func(e event.DeleteEvent) bool { return false },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		},
	); err != nil {
		return fmt.Errorf("unable to watch Namespaces: %w", err)
	}

	configMap := &metav1.PartialObjectMetadata{}
	configMap.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	if err := rootcaController.Watch(
		&source.Kind{Type: configMap},
		&handler.EnqueueRequestForOwner{OwnerType: &corev1.Namespace{}},
		predicate.Funcs{
			CreateFunc:  func(e event.CreateEvent) bool { return false },
			UpdateFunc:  func(e event.UpdateEvent) bool { return isRelevantConfigMap(e.ObjectNew) },
			DeleteFunc:  func(e event.DeleteEvent) bool { return isRelevantConfigMap(e.Object) },
			GenericFunc: func(e event.GenericEvent) bool { return false },
		},
	); err != nil {
		return fmt.Errorf("unable to watch ConfigMaps: %w", err)
	}

	return nil
}

func isRelevantConfigMap(obj client.Object) bool {
	return obj.GetName() == RootCACertConfigMapName && obj.GetAnnotations()[DescriptionAnnotation] == ""
}

func AddToManager(mgr manager.Manager) error {
	return AddToManagerWithOptions(mgr, defaultControllerConfig)
}

func isActive(obj client.Object) bool {
	namespace, ok := obj.(*corev1.Namespace)
	if !ok {
		return false
	}

	return namespace.Status.Phase == corev1.NamespaceActive
}

// AddFlags adds the needed command line flags to the given FlagSet.
func (o *ControllerOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.rootCAPath, "root-ca-file", "", "path to a file containing the root ca bundle")
	fs.IntVar(&o.maxConcurrentWorkers, "rootcapublisher-max-concurrent-workers", 0, "number of worker threads for concurrent rootcapublisher reconciliation of resources")
}

// Complete completes the given command line flags and set the defaultControllerConfig accordingly.
func (o *ControllerOptions) Complete() error {
	defaultControllerConfig = ControllerConfig{
		RootCAPath:           o.rootCAPath,
		MaxConcurrentWorkers: o.maxConcurrentWorkers,
	}
	return nil
}

// Completed returns the completed ControllerConfig.
func (o *ControllerOptions) Completed() *ControllerConfig {
	return &defaultControllerConfig
}
