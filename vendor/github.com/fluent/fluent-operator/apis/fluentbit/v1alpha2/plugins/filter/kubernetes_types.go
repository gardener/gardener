package filter

import (
	"fmt"

	"github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// Kubernetes filter allows to enrich your log files with Kubernetes metadata.
type Kubernetes struct {
	plugins.CommonParams `json:",inline"`
	// Set the buffer size for HTTP client when reading responses from Kubernetes API server.
	// +kubebuilder:validation:Pattern:="^\\d+(k|K|KB|kb|m|M|MB|mb|g|G|GB|gb)?$"
	BufferSize string `json:"bufferSize,omitempty"`
	// API Server end-point
	KubeURL string `json:"kubeURL,omitempty"`
	// CA certificate file
	KubeCAFile string `json:"kubeCAFile,omitempty"`
	// Absolute path to scan for certificate files
	KubeCAPath string `json:"kubeCAPath,omitempty"`
	// Token file
	KubeTokenFile string `json:"kubeTokenFile,omitempty"`
	// When the source records comes from Tail input plugin,
	// this option allows to specify what's the prefix used in Tail configuration.
	KubeTagPrefix string `json:"kubeTagPrefix,omitempty"`
	// When enabled, it checks if the log field content is a JSON string map,
	// if so, it append the map fields as part of the log structure.
	MergeLog *bool `json:"mergeLog,omitempty"`
	// When Merge_Log is enabled, the filter tries to assume the log field from the incoming message is a JSON string message
	// and make a structured representation of it at the same level of the log field in the map.
	// Now if Merge_Log_Key is set (a string name), all the new structured fields taken from the original log content are inserted under the new key.
	MergeLogKey string `json:"mergeLogKey,omitempty"`
	// When Merge_Log is enabled, trim (remove possible \n or \r) field values.
	MergeLogTrim *bool `json:"mergeLogTrim,omitempty"`
	// Optional parser name to specify how to parse the data contained in the log key. Recommended use is for developers or testing only.
	MergeParser string `json:"mergeParser,omitempty"`
	// When Keep_Log is disabled, the log field is removed
	// from the incoming message once it has been successfully merged
	// (Merge_Log must be enabled as well).
	KeepLog *bool `json:"keepLog,omitempty"`
	// Debug level between 0 (nothing) and 4 (every detail).
	TLSDebug *int32 `json:"tlsDebug,omitempty"`
	// When enabled, turns on certificate validation when connecting to the Kubernetes API server.
	TLSVerify *bool `json:"tlsVerify,omitempty"`
	// When enabled, the filter reads logs coming in Journald format.
	UseJournal *bool `json:"useJournal,omitempty"`
	// Set an alternative Parser to process record Tag and extract pod_name, namespace_name, container_name and docker_id.
	// The parser must be registered in a parsers file (refer to parser filter-kube-test as an example).
	RegexParser string `json:"regexParser,omitempty"`
	// Allow Kubernetes Pods to suggest a pre-defined Parser
	// (read more about it in Kubernetes Annotations section)
	K8SLoggingParser *bool `json:"k8sLoggingParser,omitempty"`
	// Allow Kubernetes Pods to exclude their logs from the log processor
	// (read more about it in Kubernetes Annotations section).
	K8SLoggingExclude *bool `json:"k8sLoggingExclude,omitempty"`
	// Include Kubernetes resource labels in the extra metadata.
	Labels *bool `json:"labels,omitempty"`
	// Include Kubernetes resource annotations in the extra metadata.
	Annotations *bool `json:"annotations,omitempty"`
	// If set, Kubernetes meta-data can be cached/pre-loaded from files in JSON format in this directory,
	// named as namespace-pod.meta
	KubeMetaPreloadCacheDir string `json:"kubeMetaPreloadCacheDir,omitempty"`
	// If set, use dummy-meta data (for test/dev purposes)
	DummyMeta *bool `json:"dummyMeta,omitempty"`
}

func (_ *Kubernetes) Name() string {
	return "kubernetes"
}

func (k *Kubernetes) Params(_ plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	err := k.AddCommonParams(kvs)
	if err != nil {
		return kvs, err
	}
	if k.BufferSize != "" {
		kvs.Insert("Buffer_Size", k.BufferSize)
	}
	if k.KubeURL != "" {
		kvs.Insert("Kube_URL", k.KubeURL)
	}
	if k.KubeCAFile != "" {
		kvs.Insert("Kube_CA_File", k.KubeCAFile)
	}
	if k.KubeCAPath != "" {
		kvs.Insert("Kube_CA_Path", k.KubeCAPath)
	}
	if k.KubeTokenFile != "" {
		kvs.Insert("Kube_Token_File", k.KubeTokenFile)
	}
	if k.KubeTagPrefix != "" {
		kvs.Insert("Kube_Tag_Prefix", k.KubeTagPrefix)
	}
	if k.MergeLog != nil {
		kvs.Insert("Merge_Log", fmt.Sprint(*k.MergeLog))
	}
	if k.MergeLogKey != "" {
		kvs.Insert("Merge_Log_Key", k.MergeLogKey)
	}
	if k.MergeLogTrim != nil {
		kvs.Insert("Merge_Log_Trim", fmt.Sprint(*k.MergeLogTrim))
	}
	if k.MergeParser != "" {
		kvs.Insert("Merge_Parser", k.MergeParser)
	}
	if k.KeepLog != nil {
		kvs.Insert("Keep_Log", fmt.Sprint(*k.KeepLog))
	}
	if k.TLSDebug != nil {
		kvs.Insert("tls.debug", fmt.Sprint(*k.TLSDebug))
	}
	if k.TLSVerify != nil {
		kvs.Insert("tls.verify", fmt.Sprint(*k.TLSVerify))
	}
	if k.UseJournal != nil {
		kvs.Insert("Use_Journal", fmt.Sprint(*k.UseJournal))
	}
	if k.RegexParser != "" {
		kvs.Insert("Regex_Parser", k.RegexParser)
	}
	if k.K8SLoggingParser != nil {
		kvs.Insert("K8S-Logging.Parser", fmt.Sprint(*k.K8SLoggingParser))
	}
	if k.K8SLoggingExclude != nil {
		kvs.Insert("K8S-Logging.Exclude", fmt.Sprint(*k.K8SLoggingExclude))
	}
	if k.Labels != nil {
		kvs.Insert("Labels", fmt.Sprint(*k.Labels))
	}
	if k.Annotations != nil {
		kvs.Insert("Annotations", fmt.Sprint(*k.Annotations))
	}
	if k.KubeMetaPreloadCacheDir != "" {
		kvs.Insert("Kube_meta_preload_cache_dir", k.KubeMetaPreloadCacheDir)
	}
	if k.DummyMeta != nil {
		kvs.Insert("Dummy_Meta", fmt.Sprint(*k.DummyMeta))
	}
	return kvs, nil
}
