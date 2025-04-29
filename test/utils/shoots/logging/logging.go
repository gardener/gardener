package logging

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/hashicorp/go-multierror"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/gardener/gardener/test/framework"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultPollInterval = 5 * time.Second

	// LoggingUserName is the admin user name for the vali instance of a shoot
	valiLogging = "vali"
	valiPort    = 3100
)

func GetLogCountFromSearchResponse(search *framework.SearchResponse) (int, error) {
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

// GetValiLogs gets logs from the last 1 hour for <key>, <value> from the vali instance in <valiNamespace>
func GetValiLogs(ctx context.Context, valiLabels map[string]string, valiNamespace, key, value string, client kubernetes.Interface) (*framework.SearchResponse, error) {
	valiLabelsSelector := labels.SelectorFromSet(valiLabels)

	query := fmt.Sprintf("query=count_over_time({%s=~\"%s\"}[1h])", key, value)

	var stdout io.Reader
	var err error
	stdout, _, err = framework.PodExecByLabel(ctx, client, valiNamespace, valiLabelsSelector, valiLogging,
		"wget", "http://localhost:"+strconv.Itoa(valiPort)+"/vali/api/v1/query", "-O-", "--post-data="+query,
	)
	if err != nil {
		return nil, err
	}

	search := &framework.SearchResponse{}

	if err := json.NewDecoder(stdout).Decode(search); err != nil {
		return nil, err
	}

	return search, nil
}

// DumpLogsForPodsWithLabelsInNamespace prints the logs of pods in the given namespace selected by the given list options.
func DumpLogsForPodsWithLabelsInNamespace(ctx context.Context, k8sClient kubernetes.Interface, log logr.Logger, namespace string, opts ...client.ListOption) error {
	pods := &corev1.PodList{}
	opts = append(opts, client.InNamespace(namespace))
	if err := k8sClient.Client().List(ctx, pods, opts...); err != nil {
		return err
	}

	var result error
	for _, pod := range pods.Items {
		if err := DumpLogsForPodInNamespace(ctx, k8sClient, log, namespace, pod.Name, &corev1.PodLogOptions{}); err != nil {
			result = multierror.Append(result, err)
		}
	}
	return result
}

// DumpLogsForPodInNamespace prints the logs of the pod with the given namespace and name.
func DumpLogsForPodInNamespace(ctx context.Context, k8sClient kubernetes.Interface, log logr.Logger, namespace, name string, options *corev1.PodLogOptions) error {
	log = log.WithValues("pod", client.ObjectKey{Namespace: namespace, Name: name})
	log.Info("Dumping logs for corev1.Pod")

	podIf := k8sClient.Kubernetes().CoreV1().Pods(namespace)
	logs, err := kubernetesutils.GetPodLogs(ctx, podIf, name, options)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(bytes.NewReader(logs))
	for scanner.Scan() {
		log.Info(scanner.Text()) //nolint:logcheck
	}

	return nil
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

func EnsureNoValiLogsFrameworkLess(ctx context.Context, log logr.Logger, valiLabels map[string]string, valiNamespace, key, value string, c kubernetes.Interface) error {
	time.Sleep(10 * time.Second)

	search, err := GetValiLogs(ctx, valiLabels, valiNamespace, key, value, c)
	if err != nil {
		return fmt.Errorf("error when trying to fetch logs from Vali: %w", err)
	}

	count, err := GetLogCountFromSearchResponse(search)
	if err != nil {
		return fmt.Errorf("error when trying to get log count from Vali: %w", err)
	}

	if count > 0 {
		return fmt.Errorf("found logs in Vali for %s=%s when they were unexpected", key, value)
	}

	log = log.WithValues("key", key, "value", value)
	log.Info("Did not receive logs for non-gardener pod. As expected.")
	return nil
}

func EnsureNoValiLogs(ctx context.Context, shootFramework *framework.ShootFramework, valiLabels map[string]string, valiNamespace, key, value string, c kubernetes.Interface) error {
	time.Sleep(10 * time.Second)

	search, err := shootFramework.GetValiLogs(ctx, valiLabels, valiNamespace, key, value, c)
	if err != nil {
		return fmt.Errorf("error when trying to fetch logs from Vali: %w", err)
	}

	count, err := GetLogCountFromSearchResponse(search)
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

func EnsureValiLogsFrameworkLess(ctx context.Context, interval time.Duration, log logr.Logger, valiLabels map[string]string, valiNamespace, key, value string, c kubernetes.Interface) error {
	err := retry.Until(ctx, interval, func(ctx context.Context) (done bool, err error) {
		search, err := GetValiLogs(ctx, valiLabels, valiNamespace, key, value, c)
		if err != nil {
			return retry.SevereError(err)
		}

		count, err := GetLogCountFromSearchResponse(search)
		if err != nil {
			return retry.SevereError(fmt.Errorf("error when trying to get log count from Vali: %w", err))
		}

		if count == 0 {
			return retry.MinorError(fmt.Errorf("no logs found in Vali for %s=%s", key, value))
		}

		log = log.WithValues("key", key, "value", value)
		log.Info("Received logs")
		return retry.Ok()
	})

	return err
}

func EnsureValiLogs(ctx context.Context, interval time.Duration, shootFramework *framework.ShootFramework, valiLabels map[string]string, valiNamespace, key, value string, c kubernetes.Interface) error {
	err := retry.Until(ctx, interval, func(ctx context.Context) (done bool, err error) {
		search, err := shootFramework.GetValiLogs(ctx, valiLabels, valiNamespace, key, value, c)
		if err != nil {
			return retry.SevereError(err)
		}

		count, err := GetLogCountFromSearchResponse(search)
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
