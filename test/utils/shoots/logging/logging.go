// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

const (
	valiLogging = "vali"
	valiPort    = 3100
)

var (
	errNoRunningPodsFound = errors.New("no running pods were found")
)

// SearchResponse represents the response from a search query to vali
type SearchResponse struct {
	Data struct {
		Result []struct {
			Values []any `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

// GetPodsByLabels fetches all pods with the desired set of labels <labelsMap>
func GetPodsByLabels(ctx context.Context, labelsSelector labels.Selector, c kubernetes.Interface, namespace string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	err := c.Client().List(ctx, podList,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: labelsSelector})
	if err != nil {
		return nil, err
	}
	return podList, nil
}

// GetFirstRunningPodWithLabels fetches the first running pod with the desired set of labels <labelsMap>
func GetFirstRunningPodWithLabels(ctx context.Context, labelsMap labels.Selector, namespace string, client kubernetes.Interface) (*corev1.Pod, error) {
	var (
		podList *corev1.PodList
		err     error
	)
	podList, err = GetPodsByLabels(ctx, labelsMap, client, namespace)
	if err != nil {
		return nil, err
	}
	if len(podList.Items) == 0 {
		return nil, errNoRunningPodsFound
	}

	for _, pod := range podList.Items {
		if health.IsPodReady(&pod) {
			return &pod, nil
		}
	}

	return nil, errNoRunningPodsFound
}

// PodExecByLabel executes a command inside pods filtered by label
func PodExecByLabel(ctx context.Context, client kubernetes.Interface, namespace string, podLabels labels.Selector, podContainer string, command ...string) (io.Reader, io.Reader, error) {
	pod, err := GetFirstRunningPodWithLabels(ctx, podLabels, namespace, client)
	if err != nil {
		return nil, nil, err
	}

	return client.PodExecutor().Execute(ctx, pod.Namespace, pod.Name, podContainer, command...)
}

// GetLogCountFromSearchResponse extracts the log count from the search response.
func GetLogCountFromSearchResponse(search *SearchResponse) int {
	total := 0
	for _, result := range search.Data.Result {
		total += len(result.Values)
	}

	return total
}

// GetValiLogs gets logs from the last 1 hour for <key>, <value> from the vali instance in <valiNamespace>
func GetValiLogs(ctx context.Context, valiLabels map[string]string, valiNamespace, key, value string, client kubernetes.Interface) (*SearchResponse, error) {
	valiLabelsSelector := labels.SelectorFromSet(valiLabels)

	query := fmt.Sprintf("query={%s=~\"%s\"}", key, value)

	var stdout io.Reader
	var err error
	stdout, _, err = PodExecByLabel(ctx, client, valiNamespace, valiLabelsSelector, valiLogging,
		"wget", "http://localhost:"+strconv.Itoa(valiPort)+"/vali/api/v1/query_range?limit=200", "-O-", "--post-data="+query,
	)
	if err != nil {
		return nil, err
	}

	search := &SearchResponse{}

	if err := json.NewDecoder(stdout).Decode(search); err != nil {
		return nil, err
	}

	return search, nil
}
