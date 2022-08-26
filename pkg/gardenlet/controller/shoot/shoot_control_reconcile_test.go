package shoot

import (
	"time"

	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation/shoot"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/component-base/featuregate"
)

var _ = Describe("shoot control reconcile", func() {

	Describe("get etcd deploy timeout", func() {
		var (
			fg             = featuregate.NewFeatureGate()
			s              *shoot.Shoot
			defaultTimeout time.Duration
		)

		BeforeEach(func() {
			Expect(fg.Add(features.GetFeatures(features.HAControlPlanes))).To(Succeed())
			s = &shoot.Shoot{
				ControlPlane: nil,
			}
			defaultTimeout = 30 * time.Second
		})

		Context("deploy timeout for etcd in non-ha shoot", func() {
			It("should return a default timeout", func() {
				Expect(getEtcdDeployTimeout(s, defaultTimeout)).To(Equal(defaultTimeout))
			})
		})
	})

})
