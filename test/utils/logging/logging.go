// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	istiov1 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
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

	// Retry potentially multiple times as vali may be evicted by VPA.
	Eventually(ctx, func() error {
		stdout, _, err = PodExecByLabel(ctx, client, valiNamespace, valiLabelsSelector, valiLogging,
			"wget", "http://localhost:"+strconv.Itoa(valiPort)+"/vali/api/v1/query_range?limit=200", "-O-", "--post-data="+query,
		)
		return err
	}, 45*time.Second, 1*time.Second).Should(Succeed())

	search := &SearchResponse{}

	if err := json.NewDecoder(stdout).Decode(search); err != nil {
		return nil, err
	}

	return search, nil
}

// vlCountResponse is the JSON response from a VictoriaLogs stats count query
type vlCountResponse struct {
	Total string `json:"total"`
}

// victoriaLogsConn holds the resolved connection parameters for a VictoriaLogs ingress endpoint.
type victoriaLogsConn struct {
	host       string
	httpClient *http.Client
	username   string
	password   string
}

// GetVictoriaLogsCount counts logs matching key=~value with timestamps at or after since.
// It uses VictoriaLogs instance in <namespace>, discovering its VirtualService by the app label.
func GetVictoriaLogsCount(ctx context.Context, since time.Time, namespace, key, value string, k8sClient kubernetes.Interface) (int, error) {
	params := url.Values{}
	params.Set("query", fmt.Sprintf("%q:~%q | stats count() as total", key, value))
	params.Set("start", since.UTC().Format(time.RFC3339Nano))
	return getVictoriaLogsCount(ctx, namespace, params, k8sClient)
}

// QueryVictoriaLogsStatus discovers the VictoriaLogs ingress host and CA pool in <namespace>, sends
// a single query request with the given credentials (pass empty strings for no auth), and returns
// the HTTP status code.
func QueryVictoriaLogsStatus(ctx context.Context, namespace, username, password string, k8sClient kubernetes.Interface) (int, error) {
	vs, err := getVictoriaLogsVirtualService(ctx, namespace, k8sClient)
	if err != nil {
		return 0, err
	}

	caPool, err := victoriaLogsIngressCAPool(ctx, namespace, caSecretNameForAuthSecret(vs.Labels[v1beta1constants.LabelBasicAuthSecretName]), k8sClient)
	if err != nil {
		return 0, err
	}

	conn := resolveVictoriaLogsConn(vs, username, password, caPool)
	defer conn.httpClient.CloseIdleConnections()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://"+conn.host+"/select/logsql/query",
		strings.NewReader("query=*&start=now-1m&limit=1"),
	)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := conn.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func getVictoriaLogsCount(ctx context.Context, namespace string, params url.Values, k8sClient kubernetes.Interface) (int, error) {
	vs, err := getVictoriaLogsVirtualService(ctx, namespace, k8sClient)
	if err != nil {
		return 0, err
	}

	authSecretName := vs.Labels[v1beta1constants.LabelBasicAuthSecretName]
	caSecretName := caSecretNameForAuthSecret(authSecretName)

	username, password, err := getVictoriaLogsCredentials(ctx, vs, k8sClient)
	if err != nil {
		return 0, err
	}

	caPool, err := victoriaLogsIngressCAPool(ctx, namespace, caSecretName, k8sClient)
	if err != nil {
		return 0, err
	}

	conn := resolveVictoriaLogsConn(vs, username, password, caPool)
	defer conn.httpClient.CloseIdleConnections()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://"+conn.host+"/select/logsql/query",
		strings.NewReader(params.Encode()),
	)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(conn.username, conn.password)
	resp, err := conn.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return decodeVictoriaLogsJSON(bytes.NewReader(body))
}

// resolveVictoriaLogsConn builds a connection configuration from the VirtualService host
// and the supplied credentials and CA pool.
func resolveVictoriaLogsConn(vs *istiov1.VirtualService, username, password string, caPool *x509.CertPool) *victoriaLogsConn {
	return &victoriaLogsConn{
		host: vs.Spec.Hosts[0],
		httpClient: &http.Client{Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: caPool},
		}},
		username: username,
		password: password,
	}
}

func getVictoriaLogsVirtualService(ctx context.Context, namespace string, k8sClient kubernetes.Interface) (*istiov1.VirtualService, error) {
	vsList := &istiov1.VirtualServiceList{}
	if err := k8sClient.Client().List(ctx, vsList,
		client.InNamespace(namespace),
		client.MatchingLabels{v1beta1constants.LabelApp: "victoria-logs"},
	); err != nil {
		return nil, fmt.Errorf("failed to list VictoriaLogs VirtualServices in %s: %w", namespace, err)
	}

	if len(vsList.Items) != 1 {
		return nil, fmt.Errorf("expected exactly one VictoriaLogs VirtualService in %s, found %d", namespace, len(vsList.Items))
	}

	vs := vsList.Items[0]
	if len(vs.Spec.Hosts) == 0 {
		return nil, fmt.Errorf("%s/%s VirtualService has no hosts", namespace, vs.Name)
	}

	return vs, nil
}

func getVictoriaLogsCredentials(ctx context.Context, vs *istiov1.VirtualService, k8sClient kubernetes.Interface) (string, string, error) {
	authSecretName := vs.Labels[v1beta1constants.LabelBasicAuthSecretName]
	if authSecretName == "" {
		return "", "", fmt.Errorf("%s/%s VirtualService has no %s label", vs.Namespace, vs.Name, v1beta1constants.LabelBasicAuthSecretName)
	}

	authSecretManaged, err := strconv.ParseBool(vs.Labels[v1beta1constants.LabelBasicAuthSecretManaged])
	if err != nil {
		return "", "", fmt.Errorf("%s/%s VirtualService has an invalid %s label: %w", vs.Namespace, vs.Name, v1beta1constants.LabelBasicAuthSecretManaged, err)
	}

	return victoriaLogsIngressCredentials(ctx, vs.Namespace, authSecretName, authSecretManaged, k8sClient)
}

// caSecretNameForAuthSecret returns the secrets-manager name label for the CA secret that corresponds to the given auth secret.
func caSecretNameForAuthSecret(authSecretName string) string {
	switch {
	case authSecretName == v1beta1constants.SecretNameObservabilityIngress:
		return operatorv1alpha1.SecretNameCARuntime
	case strings.HasPrefix(authSecretName, "seed-"):
		return v1beta1constants.SecretNameCASeed
	default:
		return v1beta1constants.SecretNameCACluster
	}
}

// victoriaLogsIngressCredentials resolves managed secrets by their name label and unmanaged secrets by their object name.
func victoriaLogsIngressCredentials(ctx context.Context, namespace, authSecretName string, authSecretManaged bool, k8sClient kubernetes.Interface) (username, password string, err error) {
	if !authSecretManaged {
		secret := &corev1.Secret{}
		if err := k8sClient.Client().Get(ctx, types.NamespacedName{Namespace: namespace, Name: authSecretName}, secret); err != nil {
			return "", "", fmt.Errorf("failed to get %s/%s secret: %w", namespace, authSecretName, err)
		}
		return string(secret.Data["username"]), string(secret.Data["password"]), nil
	}

	secretList := &corev1.SecretList{}
	if err := k8sClient.Client().List(ctx, secretList,
		client.InNamespace(namespace),
		client.MatchingLabels{"name": authSecretName},
	); err != nil {
		return "", "", fmt.Errorf("failed to list %s secrets: %w", authSecretName, err)
	}

	if len(secretList.Items) == 0 {
		return "", "", fmt.Errorf("%s secret not found", authSecretName)
	}

	secret := secretList.Items[0]
	return string(secret.Data["username"]), string(secret.Data["password"]), nil
}

// victoriaLogsIngressCAPool extends the system trust pool with all secrets matching the selected CA name label.
func victoriaLogsIngressCAPool(ctx context.Context, namespace, caSecretName string, k8sClient kubernetes.Interface) (*x509.CertPool, error) {
	secretList := &corev1.SecretList{}
	if err := k8sClient.Client().List(ctx, secretList,
		client.InNamespace(namespace),
		client.MatchingLabels{"name": caSecretName},
	); err != nil {
		return nil, fmt.Errorf("failed to list %s secrets: %w", caSecretName, err)
	}

	if len(secretList.Items) == 0 {
		return nil, fmt.Errorf("%s secret not found", caSecretName)
	}

	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}

	for _, secret := range secretList.Items {
		if !pool.AppendCertsFromPEM(secret.Data["ca.crt"]) {
			return nil, fmt.Errorf("failed to parse CA certificate in %s/%s", namespace, secret.Name)
		}
	}

	return pool, nil
}

// decodeVictoriaLogsJSON reads the total count from a VictoriaLogs stats response.
func decodeVictoriaLogsJSON(r io.Reader) (int, error) {
	var entry vlCountResponse

	if err := json.NewDecoder(r).Decode(&entry); err != nil {
		return 0, err
	}

	return strconv.Atoi(entry.Total)
}
