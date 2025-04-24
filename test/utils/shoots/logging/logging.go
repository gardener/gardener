package logging

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/gardener/gardener/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetLogCountFromResult(search *framework.SearchResponse) (int, error) {
	var totalLogs int
	for _, result := range search.Data.Result {
		currentStr, ok := result.Value[1].(string)
		if !ok {
			return totalLogs, fmt.Errorf("Data.Result.Value[1] is not a string")
		}
		current, err := strconv.Atoi(currentStr)
		if err != nil {
			return totalLogs, fmt.Errorf("Data.Result.Value[1] string is not parsable to integer")
		}
		totalLogs += current
	}
	return totalLogs, nil
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

func EnsureNoValiLogs(ctx context.Context, shootFramework *framework.ShootFramework, valiLabels map[string]string, valiNamespace, key, value string, c kubernetes.Interface) error {
	time.Sleep(10 * time.Second)

	search, err := shootFramework.GetValiLogs(ctx, valiLabels, valiNamespace, key, value, c)
	if err != nil {
		return fmt.Errorf("error when trying to fetch logs from Vali: %w", err)
	}

	count, err := GetLogCountFromResult(search)
	if err != nil {
		return fmt.Errorf("error when trying to get log count from Vali: %w", err)
	}

	if count > 0 {
		return fmt.Errorf("found logs in Vali for %s=%s when they were unexpected", key, value)
	}

	log := shootFramework.Logger.WithValues("key", key, "value", value)
	log.Info("Did not receive logs for non-gardener pod. As expected.")
	return nil
}

func EnsureValiLogs(ctx context.Context, interval time.Duration, shootFramework *framework.ShootFramework, valiLabels map[string]string, valiNamespace, key, value string, c kubernetes.Interface) error {
	err := retry.Until(ctx, interval, func(ctx context.Context) (done bool, err error) {
		search, err := shootFramework.GetValiLogs(ctx, valiLabels, valiNamespace, key, value, c)
		if err != nil {
			return retry.SevereError(err)
		}

		count, err := GetLogCountFromResult(search)
		if err != nil {
			return retry.SevereError(fmt.Errorf("error when trying to get log count from Vali: %w", err))
		}

		if count == 0 {
			return retry.MinorError(fmt.Errorf("no logs found in Vali for %s=%s", key, value))
		}

		log := shootFramework.Logger.WithValues("key", key, "value", value)
		log.Info("Received logs")
		return retry.Ok()
	})

	return err
}
