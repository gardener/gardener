package alicloudbotanist

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"
)

func TestRefreshCloudProviderConfig(t *testing.T) {
	curMap := map[string]string{
		common.CloudProviderConfigMapKey: `{
			"Global":
			{
			  "kubernetesClusterTag":"my-k8s-test",
			  "vpcid":"vpc-2zehpnv9w5escf1hfqjsg",
			  "zoneID":"cn-beijing-f",
			  "region":"cn-beijing",
			  "vswitchid":"vsw-2ze3a4pi0j4wbt39g8r8i",
			  "accessKeyID":"ABC",
			  "accessKeySecret":"ABCD"
			}
		}
		`,
	}

	s := shoot.Shoot{
		Secret: &corev1.Secret{},
	}

	b := &AlicloudBotanist{
		Operation:         &operation.Operation{},
		CloudProviderName: "alicloud",
	}
	b.Shoot = &s
	b.Shoot.Secret.Data = map[string][]byte{
		AccessKeyID:     []byte("123"),
		AccessKeySecret: []byte("1234"),
	}
	m2 := b.RefreshCloudProviderConfig(curMap)

	expected := `{"Global":{"KubernetesClusterTag":"my-k8s-test","uid":"","vpcid":"vpc-2zehpnv9w5escf1hfqjsg","region":"cn-beijing","zoneid":"cn-beijing-f","vswitchid":"vsw-2ze3a4pi0j4wbt39g8r8i","accessKeyID":"MTIz","accessKeySecret":"MTIzNA=="}}`

	if expected != m2[common.CloudProviderConfigMapKey] {
		t.Errorf("Expected: %s, Actual: %s", expected, m2[common.CloudProviderConfigMapKey])
	}
}
