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
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("APIServerProxy", func() {
	var (
		ctrl *gomock.Controller
		ctx  = context.TODO()

		seedVersion = "1.26.0"

		shoot *gardencorev1beta1.Shoot

		gardenClient  client.Client
		seedClient    client.Client
		seedClientSet kubernetes.Interface

		internalClusterDomain = "internal.foo.bar.com"
		externalClusterDomain = "external.foo.bar.com"

		advertiseIPAddress = "10.2.170.21"

		botanist *Botanist
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

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

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.Shoot{}).WithObjects(shoot).Build()
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		seedClientSet = fake.NewClientSetBuilder().WithClient(seedClient).WithVersion(seedVersion).Build()

		botanist = &Botanist{
			Operation: &operation.Operation{
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
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DeployAPIServerProxy", func() {
		It("should deploy apiserverproxy and set ShootAPIServerProxyUsesHTTPProxy constraint", func() {
			comp, err := botanist.DefaultAPIServerProxy()
			Expect(err).NotTo(HaveOccurred())
			Expect(comp).ToNot(BeNil())

			botanist.Shoot.Components.SystemComponents.APIServerProxy = comp

			Expect(botanist.DeployAPIServerProxy(ctx)).To(Succeed())

			Expect(botanist.GardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())

			Expect(shoot.Status.Constraints).To(ContainCondition(
				OfType(gardencorev1beta1.ShootAPIServerProxyUsesHTTPProxy),
				WithStatus(gardencorev1beta1.ConditionTrue),
				WithReason("ApiserverProxyUsesHTTPProxy"),
			))
		})
	})
})
