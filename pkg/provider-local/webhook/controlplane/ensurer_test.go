// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package controlplane_test

import (
	"context"

	"github.com/coreos/go-systemd/v22/unit"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionscontextwebhook "github.com/gardener/gardener/extensions/pkg/webhook/context"
	"github.com/gardener/gardener/extensions/pkg/webhook/controlplane/genericmutator"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	"github.com/gardener/gardener/pkg/provider-local/webhook/controlplane"
)

var _ = Describe("Ensurer", func() {
	const (
		namespace       = "shoot--local--foo"
		workloadIDVol   = "workload-identity"
		workloadIDMount = "/var/run/secrets/gardener.cloud/workload-identity"
	)

	var (
		ctx     = context.Background()
		gctx    extensionscontextwebhook.GardenContext
		deploy  *appsv1.Deployment
		cluster *extensionscontroller.Cluster
	)

	BeforeEach(func() {
		cluster = &extensionscontroller.Cluster{
			Shoot: &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{Workers: []gardencorev1beta1.Worker{{Name: "worker"}}},
				},
			},
		}
		gctx = extensionscontextwebhook.NewInternalGardenContext(cluster)

		deploy = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "machine-controller-manager", Namespace: namespace},
		}
	})

	newEnsurer := func(secret *corev1.Secret) genericmutator.Ensurer {
		c := fake.NewClientBuilder().WithObjects(secret).Build()
		return controlplane.NewEnsurer(c, &rest.Config{}, log.Log)
	}

	cloudProviderSecret := func(withWILabel bool) *corev1.Secret {
		s := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.SecretNameCloudProvider, Namespace: namespace},
			Data:       map[string][]byte{securityv1alpha1constants.DataKeyToken: []byte("a-token")},
		}
		if withWILabel {
			s.Labels = map[string]string{securityv1alpha1constants.LabelPurpose: securityv1alpha1constants.LabelPurposeWorkloadIdentityTokenRequestor}
		}
		return s
	}

	volumeNames := func(d *appsv1.Deployment) []string {
		var names []string
		for _, v := range d.Spec.Template.Spec.Volumes {
			names = append(names, v.Name)
		}
		return names
	}

	Describe("#EnsureMachineControllerManagerDeployment", func() {
		It("should project the workload identity token when the cloudprovider secret is a token requestor", func() {
			ensurer := newEnsurer(cloudProviderSecret(true))

			Expect(ensurer.EnsureMachineControllerManagerDeployment(ctx, gctx, deploy, nil)).To(Succeed())

			Expect(volumeNames(deploy)).To(ContainElement(workloadIDVol))
			var projected *corev1.Volume
			for i, v := range deploy.Spec.Template.Spec.Volumes {
				if v.Name == workloadIDVol {
					projected = &deploy.Spec.Template.Spec.Volumes[i]
				}
			}
			Expect(projected).NotTo(BeNil())
			Expect(projected.Projected).NotTo(BeNil())
			Expect(projected.Projected.Sources).To(HaveLen(1))
			Expect(projected.Projected.Sources[0].Secret.Name).To(Equal(v1beta1constants.SecretNameCloudProvider))
		})

		It("should not project the workload identity token for a plain cloudprovider secret", func() {
			ensurer := newEnsurer(cloudProviderSecret(false))

			Expect(ensurer.EnsureMachineControllerManagerDeployment(ctx, gctx, deploy, nil)).To(Succeed())

			Expect(volumeNames(deploy)).NotTo(ContainElement(workloadIDVol))
		})
	})

	Describe("#EnsureKubeletServiceUnitOptions", func() {
		var (
			ensurer     genericmutator.Ensurer
			shoot       *gardencorev1beta1.Shoot
			kubeletGctx extensionscontextwebhook.GardenContext
			unitOptions []*unit.UnitOption
		)

		BeforeEach(func() {
			ensurer = newEnsurer(cloudProviderSecret(false))
			shoot = &gardencorev1beta1.Shoot{}
			kubeletGctx = extensionscontextwebhook.NewInternalGardenContext(&extensionscontroller.Cluster{Shoot: shoot})

			unitOptions = []*unit.UnitOption{
				{Section: "Service", Name: "ExecStart", Value: `/opt/bin/kubelet \
    --config=/var/lib/kubelet/config/kubelet`},
			}
		})

		It("should add the external cloud provider flag for shoots with managed infrastructure", func(ctx SpecContext) {
			shoot.Spec.CredentialsBindingName = new("local")

			Expect(ensurer.EnsureKubeletServiceUnitOptions(ctx, kubeletGctx, nil, unitOptions, nil)).To(Equal([]*unit.UnitOption{
				{Section: "Service", Name: "ExecStart", Value: `/opt/bin/kubelet \
    --config=/var/lib/kubelet/config/kubelet \
    --cloud-provider=external`},
			}))
		})

		It("should not add the external cloud provider flag for shoots with unmanaged infrastructure", func(ctx SpecContext) {
			Expect(ensurer.EnsureKubeletServiceUnitOptions(ctx, kubeletGctx, nil, unitOptions, nil)).To(Equal([]*unit.UnitOption{
				{Section: "Service", Name: "ExecStart", Value: `/opt/bin/kubelet \
    --config=/var/lib/kubelet/config/kubelet`},
			}))
		})
	})
})
