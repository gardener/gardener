package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/gardener/gardener/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultPollInterval = 5 * time.Second

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

func GetLogCountFromSearchResponse(search *SearchResponse) int {
	total := 0
	fmt.Println("Number of results:", len(search.Data.Result))
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

// This was copied from test/testmachinery/shoots/logging/utils.go
// Vali labels variable was removed from function signature.
// EnsureValiLogsCount waits until the vali instance in <valiNamespace> receives <expected> logs for <key>=<value>
func EnsureValiLogsCount(ctx context.Context, interval time.Duration, shootFramework *framework.ShootFramework, valiLabels map[string]string, valiNamespace, key, value string, expected, delta int, c kubernetes.Interface) error {
	err := retry.Until(ctx, interval, func(ctx context.Context) (done bool, err error) {
		search, err := shootFramework.GetValiLogs(ctx, valiLabels, valiNamespace, key, value, c)
		if err != nil {
			return retry.SevereError(err)
		}
		var actual int
		for _, result := range search.Data.Result {
			currentStr, ok := result.Value[1].(string)
			if !ok {
				return retry.SevereError(fmt.Errorf("Data.Result.Value[1] is not a string for %s=%s", key, value))
			}
			current, err := strconv.Atoi(currentStr)
			if err != nil {
				return retry.SevereError(fmt.Errorf("Data.Result.Value[1] string is not parsable to integer for %s=%s", key, value))
			}
			actual += current
		}

		log := shootFramework.Logger.WithValues("expected", expected, "actual", actual)

		if expected > actual {
			log.Info("Waiting to receive all expected logs")
			return retry.MinorError(fmt.Errorf("received only %d/%d logs", actual, expected))
		} else if expected+delta < actual {
			return retry.SevereError(fmt.Errorf("expected to receive %d logs but was %d", expected, actual))
		}

		log.Info("Received logs", "delta", delta)
		return retry.Ok()
	})

	if err != nil {
		// ctx might have been cancelled already, make sure we still dump logs, so use context.Background()
		dumpLogsCtx, dumpLogsCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer dumpLogsCancel()

		shootFramework.Logger.Info("Dump Vali logs")
		if dumpError := shootFramework.DumpLogsForPodInNamespace(dumpLogsCtx, c, valiNamespace, "vali-0",
			&corev1.PodLogOptions{Container: "vali"}); dumpError != nil {
			shootFramework.Logger.Error(dumpError, "Error dumping logs for pod")
		}

		shootFramework.Logger.Info("Dump Fluent-bit logs")
		labels := client.MatchingLabels{"app": "fluent-bit"}
		if dumpError := shootFramework.DumpLogsForPodsWithLabelsInNamespace(dumpLogsCtx, c, "garden",
			labels); dumpError != nil {
			shootFramework.Logger.Error(dumpError, "Error dumping logs for pod")
		}
	}

	return err
}
