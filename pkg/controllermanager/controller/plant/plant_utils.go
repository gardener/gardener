package plant

import (
	"context"
	"fmt"
	"strings"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"github.com/pkg/errors"
	kubernetesclientset "k8s.io/client-go/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Following labels come from k8s.io/kubernetes/pkg/kubelet/apis

	// LabelZoneFailureDomain zone failure domain label
	LabelZoneFailureDomain = "failure-domain.beta.kubernetes.io/zone"
	// LabelZoneRegion zone region label
	LabelZoneRegion = "failure-domain.beta.kubernetes.io/region"
)

func newConditionOrError(oldCondition, newCondition gardencorev1alpha1.Condition, err error) gardencorev1alpha1.Condition {
	if err != nil {
		return helper.UpdatedConditionUnknownError(oldCondition, err)
	}
	return newCondition
}

func FetchCloudInfo(ctx context.Context, plantClient client.Client, discoveryClient *kubernetesclientset.Clientset, plant *gardencorev1alpha1.Plant, logger logrus.FieldLogger) (*plantStatusInfo, error) {
	if plantClient == nil || discoveryClient == nil {
		return nil, fmt.Errorf("plant clients need to be initialized first")
	}

	cloudInfo, err := GetClusterInfo(ctx, plantClient, logger)
	if err != nil {
		return nil, err
	}

	kubernetesVersionInfo, err := discoveryClient.ServerVersion()
	if err != nil {
		return nil, err
	}

	cloudInfo.k8sVersion = kubernetesVersionInfo.String()

	return cloudInfo, nil
}

// GetClusterInfo gets the kubernetes cluster zones and region by inspecting labels on nodes in the cluster.
func GetClusterInfo(ctx context.Context, cl client.Client, logger logrus.FieldLogger) (*plantStatusInfo, error) {
	var nodes = &corev1.NodeList{}

	err := cl.List(ctx, &client.ListOptions{Namespace: "garden"}, nodes)
	if err != nil {
		logger.Errorf("Failed to list nodes while getting zone names: %v", err)
		return nil, err
	}

	if len(nodes.Items) == 0 {
		return nil, fmt.Errorf("there are no nodes available in this cluster to retrieve zones and regions from")
	}

	// we are only taking the first node because all nodes that
	firstNode := nodes.Items[0]
	region, err := getRegionNameForNode(firstNode)
	if err != nil {
		return nil, err
	}

	provider := getCloudProviderForNode(firstNode.Spec.ProviderID)

	return &plantStatusInfo{
		region:    region,
		cloudType: provider,
	}, nil
}

func getCloudProviderForNode(providerID string) string {

	provider := strings.Split(providerID, "://")
	if len(provider) == 0 {
		return "<unknown>"
	}

	return provider[0]
}

// getRegionNameForNode Finds the name of the region in which a Node is running.
func getRegionNameForNode(node corev1.Node) (string, error) {
	for key, value := range node.Labels {
		if key == LabelZoneRegion {
			return value, nil
		}
	}
	return "", errors.Errorf("Region name for node %s not found. No label with key %s",
		node.Name, LabelZoneRegion)
}
