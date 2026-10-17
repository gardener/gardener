// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardenadmadmission_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/gardenadm"
	. "github.com/gardener/gardener/pkg/gardenadm/admission"
)

var _ = Describe("RunAdmissionChain", func() {
	const (
		shootNamespace    = "garden"
		cloudProfileName  = "local"
		projectName       = "my-project"
		shootName         = "my-shoot"
		machineImageName  = "gardenlinux"
		machineImageVer   = "1312.3.0"
		machineTypeName   = "standard-1"
		kubernetesVersion = "1.30.0"
		region            = "local"
	)

	var (
		ctx       = context.Background()
		log       = logr.Discard()
		resources *gardenadm.Resources
	)

	BeforeEach(func() {
		cloudProfile := &gardencorev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: cloudProfileName,
			},
			Spec: gardencorev1beta1.CloudProfileSpec{
				Type: "local",
				Kubernetes: gardencorev1beta1.KubernetesSettings{
					Versions: []gardencorev1beta1.ExpirableVersion{
						{Version: kubernetesVersion},
					},
				},
				MachineImages: []gardencorev1beta1.MachineImage{
					{
						Name: machineImageName,
						Versions: []gardencorev1beta1.MachineImageVersion{
							{
								ExpirableVersion: gardencorev1beta1.ExpirableVersion{
									Version: machineImageVer,
								},
								CRI: []gardencorev1beta1.CRI{
									{Name: gardencorev1beta1.CRINameContainerD},
								},
								Architectures: []string{"amd64"},
							},
						},
					},
				},
				MachineTypes: []gardencorev1beta1.MachineType{
					{
						Name:         machineTypeName,
						CPU:          resource.MustParse("1"),
						GPU:          resource.MustParse("0"),
						Memory:       resource.MustParse("4Gi"),
						Usable:       new(true),
						Architecture: new("amd64"),
					},
				},
				Regions: []gardencorev1beta1.Region{
					{
						Name: region,
						Zones: []gardencorev1beta1.AvailabilityZone{
							{Name: "local-a"},
						},
					},
				},
			},
		}

		project := &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: new(shootNamespace),
			},
		}

		shoot := &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: shootNamespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				CloudProfile: &gardencorev1beta1.CloudProfileReference{
					Kind: "CloudProfile",
					Name: cloudProfileName,
				},
				Region: region,
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: kubernetesVersion,
				},
				Networking: &gardencorev1beta1.Networking{
					Type:     new("calico"),
					Nodes:    new("10.250.0.0/16"),
					Pods:     new("100.96.0.0/11"),
					Services: new("100.64.0.0/13"),
				},
				Provider: gardencorev1beta1.Provider{
					Type: "local",
					Workers: []gardencorev1beta1.Worker{
						{
							Name: "worker",
							Machine: gardencorev1beta1.Machine{
								Type: machineTypeName,
								Image: &gardencorev1beta1.ShootMachineImage{
									Name:    machineImageName,
									Version: new(machineImageVer),
								},
							},
							Minimum:      1,
							Maximum:      1,
							MaxSurge:     new(intstr.FromInt32(1)),
							ControlPlane: &gardencorev1beta1.WorkerControlPlane{},
							CRI: &gardencorev1beta1.CRI{
								Name: gardencorev1beta1.CRINameContainerD,
							},
						},
					},
				},
				DNS: &gardencorev1beta1.DNS{
					Domain: new("my-shoot.example.com"),
					Providers: []gardencorev1beta1.DNSProvider{
						{
							Type:    new("unmanaged"),
							Primary: new(true),
						},
					},
				},
			},
		}

		resources = &gardenadm.Resources{
			CloudProfile: cloudProfile,
			Project:      project,
			Shoot:        shoot,
			ControllerRegistrations: []*gardencorev1beta1.ControllerRegistration{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "provider-local"},
					Spec: gardencorev1beta1.ControllerRegistrationSpec{
						Resources: []gardencorev1beta1.ControllerResource{
							{Kind: "ControlPlane", Type: "local"},
							{Kind: "Infrastructure", Type: "local"},
							{Kind: "Worker", Type: "local"},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "networking-calico"},
					Spec: gardencorev1beta1.ControllerRegistrationSpec{
						Resources: []gardencorev1beta1.ControllerResource{
							{Kind: "Network", Type: "calico"},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "os-gardenlinux"},
					Spec: gardencorev1beta1.ControllerRegistrationSpec{
						Resources: []gardencorev1beta1.ControllerResource{
							{Kind: "OperatingSystemConfig", Type: "gardenlinux"},
						},
					},
				},
			},
		}
	})

	It("should succeed for a valid minimal Shoot", func() {
		Expect(RunAdmissionChain(ctx, log, resources)).To(Succeed())
	})

	It("should set VPA enabled by default after running the chain", func() {
		Expect(RunAdmissionChain(ctx, log, resources)).To(Succeed())

		Expect(resources.Shoot.Spec.Kubernetes.VerticalPodAutoscaler).NotTo(BeNil())
		Expect(resources.Shoot.Spec.Kubernetes.VerticalPodAutoscaler.Enabled).To(BeTrue())
	})

	It("should set kubelet resource reservations after running the chain", func() {
		Expect(RunAdmissionChain(ctx, log, resources)).To(Succeed())

		Expect(resources.Shoot.Spec.Kubernetes.Kubelet).NotTo(BeNil())
		Expect(resources.Shoot.Spec.Kubernetes.Kubelet.KubeReserved).NotTo(BeNil())
	})

	It("should not enable node-local-dns by default", func() {
		Expect(RunAdmissionChain(ctx, log, resources)).To(Succeed())

		Expect(resources.Shoot.Spec.SystemComponents).To(BeNil())
	})

	It("should return an error for a Shoot referencing a kubernetes version not in the CloudProfile", func() {
		resources.Shoot.Spec.Kubernetes.Version = "9.9.9"

		err := RunAdmissionChain(ctx, log, resources)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("9.9.9"))
	})
})
