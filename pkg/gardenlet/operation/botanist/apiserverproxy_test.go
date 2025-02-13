package botanist_test

import (
	"context"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	. "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("APIServerProxy", func() {
	var (
		ctx   = context.TODO()
		shoot *gardencorev1beta1.Shoot

		botanist *Botanist
	)

	BeforeEach(func() {
		internalClusterDomain := "internal.foo.bar.com"
		externalClusterDomain := "external.foo.bar.com"
		advertiseIPAddress := "10.2.170.21"

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bar",
				Namespace: "foo",
			},
			Spec: gardencorev1beta1.ShootSpec{
				Networking: &gardencorev1beta1.Networking{
					IPFamilies: []gardencorev1beta1.IPFamily{"IPv4"},
				},
				DNS: &gardencorev1beta1.DNS{
					Domain: &externalClusterDomain,
				},
			},
			Status: gardencorev1beta1.ShootStatus{
				LastOperation: &gardencorev1beta1.LastOperation{
					Type: gardencorev1beta1.LastOperationTypeReconcile,
				},
			},
		}

		gardenClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.Shoot{}).WithObjects(shoot).Build()
		seedClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		seedClientSet := fake.NewClientSetBuilder().WithClient(seedClient).Build()

		botanist = &Botanist{
			Operation: &operation.Operation{
				Clock:              clock.RealClock{},
				GardenClient:       gardenClient,
				APIServerClusterIP: advertiseIPAddress,
				Garden: &garden.Garden{
					InternalDomain: &gardenerutils.Domain{Provider: "some-provider"},
				},
				SeedClientSet: seedClientSet,
				Shoot: &shootpkg.Shoot{
					Components: &shootpkg.Components{
						SystemComponents: &shootpkg.SystemComponents{},
					},
					InternalClusterDomain: internalClusterDomain,
					ExternalClusterDomain: &externalClusterDomain,
					ExternalDomain:        &gardenerutils.Domain{Provider: "some-external-provider"},
				},
			},
		}

		botanist.Shoot.SetInfo(shoot)
		comp, err := botanist.DefaultAPIServerProxy()
		Expect(err).NotTo(HaveOccurred())
		Expect(comp).ToNot(BeNil())

		botanist.Shoot.Components.SystemComponents.APIServerProxy = comp
	})

	Describe("#DeployAPIServerProxy", func() {
		It("should deploy apiserverproxy and set ShootAPIServerProxyUsesHTTPProxy constraint", func() {
			Expect(botanist.DeployAPIServerProxy(ctx)).To(Succeed())

			Expect(botanist.GardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())

			Expect(shoot.Status.Constraints).To(ContainCondition(
				OfType(gardencorev1beta1.ShootAPIServerProxyUsesHTTPProxy),
				WithStatus(gardencorev1beta1.ConditionTrue),
				WithReason("APIServerProxyUsesHTTPProxy"),
			))
		})
	})
})
