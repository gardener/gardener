package shoot

import (
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("shoot control reconcile", func() {

	Describe("get etcd deploy timeout", func() {
		var (
			s              *shoot.Shoot
			defaultTimeout time.Duration
		)

		BeforeEach(func() {
			s = &shoot.Shoot{}
			s.SetInfo(&gardencorev1beta1.Shoot{})
			defaultTimeout = 30 * time.Second
		})

		Context("deploy timeout for etcd in non-ha s", func() {
			It("HAControlPlanes feature is not enabled", func() {
				test.WithFeatureGate(gardenletfeatures.FeatureGate, features.HAControlPlanes, false)
				Expect(getEtcdDeployTimeout(s, defaultTimeout)).To(Equal(defaultTimeout))
			})
			It("HAControlPlanes feature is enabled but s is not marked to have HA control plane", func() {
				test.WithFeatureGate(gardenletfeatures.FeatureGate, features.HAControlPlanes, true)
				Expect(getEtcdDeployTimeout(s, defaultTimeout)).To(Equal(defaultTimeout))

			})
			It("HAControlPlanes feature is enabled and s is marked as multi-zonal", func() {
				test.WithFeatureGate(gardenletfeatures.FeatureGate, features.HAControlPlanes, true)
				s.GetInfo().Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
					HighAvailability: gardencorev1beta1.HighAvailability{FailureTolerance: gardencorev1beta1.FailureTolerance{FailureToleranceType: gardencorev1beta1.FailureToleranceTypeNode}},
				}
				Expect(getEtcdDeployTimeout(s, defaultTimeout)).To(Equal(etcd.DefaultTimeout))
			})
		})
	})

})
