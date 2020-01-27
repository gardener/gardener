package shootstate

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/gardener/gardener/pkg/apis/core"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	externalinformer "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	v1alpha1 "github.com/gardener/gardener/pkg/client/core/listers/core/v1alpha1"
	gardenclientset "github.com/gardener/gardener/pkg/client/garden/clientset/internalversion"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apiserver/pkg/admission"
)

const (
	// PluginName is the name of this admission plugin.
	PluginName = "ShootStateDeletion"
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, NewFactory)
}

// NewFactory creates a new PluginFactory.
func NewFactory(config io.Reader) (admission.Interface, error) {
	return New()
}

// ValidateShootStateDeletion contains listers and admission handler.
type ValidateShootStateDeletion struct {
	*admission.Handler
	shootStateLister v1alpha1.ShootStateLister
	gardenClient     gardenclientset.Interface
	readyFunc        admission.ReadyFunc
}

var (
	_ = admissioninitializer.WantsExternalCoreInformerFactory(&ValidateShootStateDeletion{})
	_ = admissioninitializer.WantsInternalGardenClientset(&ValidateShootStateDeletion{})

	readyFuncs = []admission.ReadyFunc{}
)

// New creates a new ShootStateDeletion admission plugin.
func New() (*ValidateShootStateDeletion, error) {
	return &ValidateShootStateDeletion{
		Handler: admission.NewHandler(admission.Delete),
	}, nil
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (d *ValidateShootStateDeletion) AssignReadyFunc(f admission.ReadyFunc) {
	d.readyFunc = f
	d.SetReadyFunc(f)
}

// SetExternalCoreInformerFactory gets the garden core informer factory and adds it.
func (d *ValidateShootStateDeletion) SetExternalCoreInformerFactory(f externalinformer.SharedInformerFactory) {
	shootStateInformer := f.Core().V1alpha1().ShootStates()
	d.shootStateLister = shootStateInformer.Lister()

	readyFuncs = append(readyFuncs, shootStateInformer.Informer().HasSynced)
}

// SetInternalGardenClientset gets the garden clientset and adds it
func (d *ValidateShootStateDeletion) SetInternalGardenClientset(c gardenclientset.Interface) {
	d.gardenClient = c
}

func (d *ValidateShootStateDeletion) waitUntilReady(attrs admission.Attributes) error {
	// Wait until the caches have been synced
	if d.readyFunc == nil {
		d.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}

	if !d.WaitForReady() {
		return admission.NewForbidden(attrs, errors.New("not yet ready to handle request"))
	}

	return nil
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (d *ValidateShootStateDeletion) ValidateInitialization() error {
	if d.shootStateLister == nil {
		return errors.New("missing ShootState lister")
	}

	return nil
}

var _ admission.ValidationInterface = &ValidateShootStateDeletion{}

// Validate makes admissions decisions based on deletion confirmation annotation.
func (d *ValidateShootStateDeletion) Validate(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) error {
	if a.GetKind().GroupKind() != core.Kind("ShootState") {
		return nil
	}

	if err := d.waitUntilReady(a); err != nil {
		return fmt.Errorf("Err while waiting for ready %v", err)
	}

	if a.GetName() == "" {
		return d.validateDeleteCollection(ctx, a)
	}

	return d.validateDelete(ctx, a)
}

func (d *ValidateShootStateDeletion) validateDeleteCollection(ctx context.Context, attrs admission.Attributes) error {
	shootStateList, err := d.shootStateLister.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, shootState := range shootStateList {
		if err := d.validateDelete(ctx, d.createAdmissionAttributes(shootState, attrs)); err != nil {
			return err
		}
	}

	return nil
}

func (d *ValidateShootStateDeletion) validateDelete(ctx context.Context, attrs admission.Attributes) error {
	_, err := d.gardenClient.Garden().Shoots(attrs.GetNamespace()).Get(attrs.GetName(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return admission.NewForbidden(attrs, fmt.Errorf("Shoot %s in namespace %s still exists", attrs.GetName(), attrs.GetNamespace()))
}

func (d *ValidateShootStateDeletion) createAdmissionAttributes(obj metav1.Object, a admission.Attributes) admission.Attributes {
	return admission.NewAttributesRecord(a.GetObject(),
		a.GetOldObject(),
		a.GetKind(),
		a.GetNamespace(),
		obj.GetName(),
		a.GetResource(),
		a.GetSubresource(),
		a.GetOperation(),
		a.GetOperationOptions(),
		a.IsDryRun(),
		a.GetUserInfo())
}
