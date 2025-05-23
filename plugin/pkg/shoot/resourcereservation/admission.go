// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcereservation

import (
	"context"
	"errors"
	"fmt"
	"io"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/admission"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	admissioninitializer "github.com/gardener/gardener/pkg/apiserver/admission/initializer"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	plugin "github.com/gardener/gardener/plugin/pkg"
	"github.com/gardener/gardener/plugin/pkg/utils"
)

const (
	_ = 1 << (iota * 10)
	kib
	mib
	gib
)

// Register registers a plugin.
func Register(plugins *admission.Plugins) {
	plugins.Register(plugin.PluginNameShootResourceReservation, func(config io.Reader) (admission.Interface, error) {
		cfg, err := LoadConfiguration(config)
		if err != nil {
			return nil, err
		}

		selector, err := metav1.LabelSelectorAsSelector(cfg.Selector)
		if err != nil {
			return nil, err
		}

		return New(cfg.UseGKEFormula, selector), nil
	})
}

// ResourceReservation contains required information to process admission requests.
type ResourceReservation struct {
	*admission.Handler
	cloudProfileLister           gardencorev1beta1listers.CloudProfileLister
	namespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister
	readyFunc                    admission.ReadyFunc

	useGKEFormula bool
	labelSelector labels.Selector
}

var (
	_ = admissioninitializer.WantsCoreInformerFactory(&ResourceReservation{})

	readyFuncs []admission.ReadyFunc
)

// New creates a new ResourceReservation admission plugin.
func New(useGKEFormula bool, labelSelector labels.Selector) admission.MutationInterface {
	return &ResourceReservation{
		Handler:       admission.NewHandler(admission.Create, admission.Update),
		useGKEFormula: useGKEFormula,
		labelSelector: labelSelector,
	}
}

// AssignReadyFunc assigns the ready function to the admission handler.
func (c *ResourceReservation) AssignReadyFunc(f admission.ReadyFunc) {
	c.readyFunc = f
	c.SetReadyFunc(f)
}

// SetCoreInformerFactory gets Lister from SharedInformerFactory.
func (c *ResourceReservation) SetCoreInformerFactory(f gardencoreinformers.SharedInformerFactory) {
	cloudProfileInformer := f.Core().V1beta1().CloudProfiles()
	c.cloudProfileLister = cloudProfileInformer.Lister()

	namespacedCloudProfileInformer := f.Core().V1beta1().NamespacedCloudProfiles()
	c.namespacedCloudProfileLister = namespacedCloudProfileInformer.Lister()

	readyFuncs = append(readyFuncs, cloudProfileInformer.Informer().HasSynced, namespacedCloudProfileInformer.Informer().HasSynced)
}

// ValidateInitialization checks whether the plugin was correctly initialized.
func (c *ResourceReservation) ValidateInitialization() error {
	if c.cloudProfileLister == nil {
		return errors.New("missing cloudProfile lister")
	}
	if c.namespacedCloudProfileLister == nil {
		return errors.New("missing namespacedCloudProfile lister")
	}
	return nil
}

// Admit injects default resource reservations into worker pools of shoot objects
func (c *ResourceReservation) Admit(_ context.Context, a admission.Attributes, _ admission.ObjectInterfaces) error {
	// Wait until the caches have been synced
	if c.readyFunc == nil {
		c.AssignReadyFunc(func() bool {
			for _, readyFunc := range readyFuncs {
				if !readyFunc() {
					return false
				}
			}
			return true
		})
	}
	if !c.WaitForReady() {
		return admission.NewForbidden(a, errors.New("not yet ready to handle request"))
	}

	switch {
	case a.GetKind().GroupKind() != core.Kind("Shoot"),
		a.GetSubresource() != "":
		return nil
	}

	shoot, ok := a.GetObject().(*core.Shoot)
	if !ok {
		return apierrors.NewInternalError(errors.New("could not convert resource into Shoot object"))
	}

	// Pass if the shoot is intended to get deleted
	if shoot.DeletionTimestamp != nil {
		return nil
	}

	if len(shoot.Spec.Provider.Workers) == 0 {
		// not relevant for workerless shoots
		return nil
	}

	if !c.useGKEFormula || !c.labelSelector.Matches(labels.Set(shoot.Labels)) {
		setStaticResourceReservationDefaults(shoot)
		return nil
	}

	if shoot.Spec.Kubernetes.Kubelet != nil && shoot.Spec.Kubernetes.Kubelet.KubeReserved != nil {
		// Inject static defaults for shoots with global resource reservations
		setStaticResourceReservationDefaults(shoot)
		return nil
	}

	cloudProfileSpec, err := utils.GetCloudProfileSpec(c.cloudProfileLister, c.namespacedCloudProfileLister, shoot)
	if err != nil {
		return apierrors.NewInternalError(fmt.Errorf("could not find referenced cloud profile: %+v", err.Error()))
	}
	machineTypeMap := buildMachineTypeMap(cloudProfileSpec)

	allErrs := field.ErrorList{}
	workersPath := field.NewPath("spec", "provider", "workers")

	for i := 0; i < len(shoot.Spec.Provider.Workers); i++ {
		workerPath := workersPath.Index(i)
		worker := &shoot.Spec.Provider.Workers[i]

		allErrs = injectResourceReservations(worker, machineTypeMap, *workerPath, allErrs)
	}

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(a.GetKind().GroupKind(), a.GetName(), allErrs)
	}

	return nil
}

func setStaticResourceReservationDefaults(shoot *core.Shoot) {
	var (
		kubeReservedMemory = resource.MustParse("1Gi")
		kubeReservedCPU    = resource.MustParse("80m")
		kubeReservedPID    = resource.MustParse("20k")
	)

	if shoot.Spec.Kubernetes.Kubelet == nil {
		shoot.Spec.Kubernetes.Kubelet = &core.KubeletConfig{}
	}
	kubelet := shoot.Spec.Kubernetes.Kubelet

	if kubelet.KubeReserved == nil {
		kubelet.KubeReserved = &core.KubeletConfigReserved{
			CPU:    &kubeReservedCPU,
			Memory: &kubeReservedMemory,
			PID:    &kubeReservedPID,
		}
	} else {
		if kubelet.KubeReserved.Memory == nil {
			kubelet.KubeReserved.Memory = &kubeReservedMemory
		}
		if kubelet.KubeReserved.CPU == nil {
			kubelet.KubeReserved.CPU = &kubeReservedCPU
		}
		if kubelet.KubeReserved.PID == nil {
			kubelet.KubeReserved.PID = &kubeReservedPID
		}
	}
}

func injectResourceReservations(worker *core.Worker, machineTypeMap map[string]gardencorev1beta1.MachineType, path field.Path, allErrs field.ErrorList) field.ErrorList {
	reservation, err := calculateResourceReservationForMachineType(machineTypeMap, worker.Machine.Type)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(path.Child("machine", "type"), worker.Machine.Type, "worker machine type unknown"))
		return allErrs
	}

	if worker.Kubernetes == nil {
		worker.Kubernetes = &core.WorkerKubernetes{}
	}
	if worker.Kubernetes.Kubelet == nil {
		worker.Kubernetes.Kubelet = &core.KubeletConfig{}
	}
	if worker.Kubernetes.Kubelet.KubeReserved == nil {
		worker.Kubernetes.Kubelet.KubeReserved = reservation
	} else {
		kubeReserved := worker.Kubernetes.Kubelet.KubeReserved
		if kubeReserved.CPU == nil {
			kubeReserved.CPU = reservation.CPU
		}
		if kubeReserved.Memory == nil {
			kubeReserved.Memory = reservation.Memory
		}
		if kubeReserved.PID == nil {
			kubeReserved.PID = reservation.PID
		}
	}
	return allErrs
}

func buildMachineTypeMap(cloudProfileSpec *gardencorev1beta1.CloudProfileSpec) map[string]gardencorev1beta1.MachineType {
	types := map[string]gardencorev1beta1.MachineType{}

	for _, machine := range cloudProfileSpec.MachineTypes {
		types[machine.Name] = machine
	}
	return types
}

func calculateResourceReservationForMachineType(machineTypeMap map[string]gardencorev1beta1.MachineType, machineType string) (*core.KubeletConfigReserved, error) {
	kubeReservedPID := resource.MustParse("20k")
	machine, ok := machineTypeMap[machineType]
	if !ok {
		return nil, fmt.Errorf("unknown machine type %v", machineType)
	}

	cpuReserved := calculateCPUReservation(machine.CPU.MilliValue())
	memoryReserved := calculateMemoryReservation(machine.Memory.Value())

	return &core.KubeletConfigReserved{
		CPU:    resource.NewMilliQuantity(cpuReserved, resource.BinarySI),
		Memory: resource.NewQuantity(memoryReserved, resource.BinarySI),
		PID:    &kubeReservedPID,
	}, nil
}

func calculateCPUReservation(cpuMilli int64) int64 {
	reservation := int64(0)
	// 6% of first core
	if cpuMilli > 0 {
		reservation += 60
	}
	// + 1% of second core
	if cpuMilli > 1000 {
		reservation += 10
	}
	// + 0.5% each for core 3 and 4
	if cpuMilli > 2000 {
		reservation += (min(cpuMilli/1000, 4) - 2) * 5
	}
	// + 0.25% for the remaining CPU cores
	if cpuMilli > 4000 {
		reservation += (cpuMilli/1000 - 4) * 5 / 2
	}

	return reservation
}

func calculateMemoryReservation(memory int64) int64 {
	reservation := int64(0)
	if memory < 1*gib {
		reservation = 255 * mib
	}
	// 25% of first 4 GB
	if memory >= 1*gib {
		reservation += min(memory, 4*gib) / 4
	}
	// 20% for additional memory between 4GB and 8GB
	if memory >= 4*gib {
		reservation += (min(memory, 8*gib) - 4*gib) / 5
	}
	// 10% for additional memory between 8GB and 16GB
	if memory >= 8*gib {
		reservation += (min(memory, 16*gib) - 8*gib) / 10
	}
	// 6% for additional memory between 16GB and 128GB
	if memory >= 16*gib {
		reservation += (min(memory, 128*gib) - 16*gib) / 100 * 6
	}
	// 2% of remaining memory
	if memory >= 128*gib {
		reservation += (memory - 128*gib) / 100 * 2
	}

	return reservation
}
