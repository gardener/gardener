// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/test/framework"
)

const (
	// LoggingUserName is the admin user name for the vali instance of a shoot
	valiLogging = "vali"
	valiPort    = 3100
)

// SearchResponse represents the response from a search query to vali
type SearchResponse struct {
	Data struct {
		Result []struct {
			Values []any `json:"values"`
		} `json:"result"`
	} `json:"data"`
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
	stdout, _, err = framework.PodExecByLabel(ctx, client, valiNamespace, valiLabelsSelector, valiLogging,
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
