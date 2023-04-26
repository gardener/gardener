package output

import (
	"fmt"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
	"github.com/fluent/fluent-operator/v2/pkg/utils"
)

// +kubebuilder:object:generate:=true

// Stackdriver is the Stackdriver output plugin, allows you to ingest your records into GCP Stackdriver. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/outputs/stackdriver**
type Stackdriver struct {
	// Path to GCP Credentials JSON file
	GoogleServiceCredentials string `json:"googleServiceCredentials,omitempty"`
	// Email associated with the service
	ServiceAccountEmail *plugins.Secret `json:"serviceAccountEmail,omitempty"`
	// Private Key associated with the service
	ServiceAccountSecret *plugins.Secret `json:"serviceAccountSecret,omitempty"`
	// Metadata Server Prefix
	MetadataServer string `json:"metadataServer,omitempty"`
	// GCP/AWS region to store data. Required if Resource is generic_node or generic_task
	Location string `json:"location,omitempty"`
	// Namespace identifier. Required if Resource is generic_node or generic_task
	Namespace string `json:"namespace,omitempty"`
	// Node identifier within the namespace. Required if Resource is generic_node or generic_task
	NodeID string `json:"nodeID,omitempty"`
	// Identifier for a grouping of tasks. Required if Resource is generic_task
	Job string `json:"job,omitempty"`
	// Identifier for a task within a namespace. Required if Resource is generic_task
	TaskID string `json:"taskID,omitempty"`
	// The GCP Project that should receive the logs
	ExportToProjectID string `json:"exportToProjectID,omitempty"`
	// Set resource types of data
	Resource string `json:"resource,omitempty"`
	// Name of the cluster that the pod is running in. Required if Resource is k8s_container, k8s_node, or k8s_pod
	K8sClusterName string `json:"k8sClusterName,omitempty"`
	// Location of the cluster that contains the pods/nodes. Required if Resource is k8s_container, k8s_node, or k8s_pod
	K8sClusterLocation string `json:"k8sClusterLocation,omitempty"`
	// Used by Stackdriver to find related labels and extract them to LogEntry Labels
	LabelsKey string `json:"labelsKey,omitempty"`
	// Optional list of comma separated of strings for key/value pairs
	Labels []string `json:"labels,omitempty"`
	// The value of this field is set as the logName field in Stackdriver
	LogNameKey string `json:"logNameKey,omitempty"`
	// Used to validate the tags of logs that when the Resource is k8s_container, k8s_node, or k8s_pod
	TagPrefix string `json:"tagPrefix,omitempty"`
	// Specify the key that contains the severity information for the logs
	SeverityKey string `json:"severityKey,omitempty"`
	// Rewrite the trace field to be formatted for use with GCP Cloud Trace
	AutoformatStackdriverTrace *bool `json:"autoformatStackdriverTrace,omitempty"`
	// Number of dedicated threads for the Stackdriver Output Plugin
	Workers *int32 `json:"workers,omitempty"`
	// A custom regex to extract fields from the local_resource_id of the logs
	CustomK8sRegex string `json:"customK8sRegex,omitempty"`
	// Optional list of comma seperated strings. Setting these fields overrides the Stackdriver monitored resource API values
	ResourceLabels []string `json:"resourceLabels,omitempty"`
}

// Name implement Section() method
func (_ *Stackdriver) Name() string {
	return "stackdriver"
}

// Params implement Section() method
func (o *Stackdriver) Params(sl plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	if o.GoogleServiceCredentials != "" {
		kvs.Insert("google_service_credentials", o.GoogleServiceCredentials)
	}
	if o.ServiceAccountEmail != nil {
		u, err := sl.LoadSecret(*o.ServiceAccountEmail)
		if err != nil {
			return nil, err
		}
		kvs.Insert("service_account_email", u)
	}
	if o.ServiceAccountSecret != nil {
		u, err := sl.LoadSecret(*o.ServiceAccountSecret)
		if err != nil {
			return nil, err
		}
		kvs.Insert("service_account_secret", u)
	}
	if o.MetadataServer != "" {
		kvs.Insert("metadata_server", o.MetadataServer)
	}
	if o.Location != "" {
		kvs.Insert("location", o.Location)
	}
	if o.Namespace != "" {
		kvs.Insert("namespace", o.Namespace)
	}
	if o.NodeID != "" {
		kvs.Insert("node_id", o.NodeID)
	}
	if o.Job != "" {
		kvs.Insert("job", o.Job)
	}
	if o.TaskID != "" {
		kvs.Insert("task_id", o.TaskID)
	}
	if o.ExportToProjectID != "" {
		kvs.Insert("export_to_project_id", o.ExportToProjectID)
	}
	if o.Resource != "" {
		kvs.Insert("resource", o.Resource)
	}
	if o.K8sClusterName != "" {
		kvs.Insert("k8s_cluster_name", o.K8sClusterName)
	}
	if o.K8sClusterLocation != "" {
		kvs.Insert("k8s_cluster_location", o.K8sClusterLocation)
	}
	if o.LabelsKey != "" {
		kvs.Insert("labels_key", o.LabelsKey)
	}
	if o.Labels != nil && len(o.Labels) > 0 {
		kvs.Insert("labels", utils.ConcatString(o.Labels, ","))
	}
	if o.LogNameKey != "" {
		kvs.Insert("log_name_key", o.LogNameKey)
	}
	if o.TagPrefix != "" {
		kvs.Insert("tag_prefix", o.TagPrefix)
	}
	if o.SeverityKey != "" {
		kvs.Insert("severity_key", o.SeverityKey)
	}
	if o.AutoformatStackdriverTrace != nil {
		kvs.Insert("autoformat_stackdriver_trace", fmt.Sprint(*o.AutoformatStackdriverTrace))
	}
	if o.Workers != nil {
		kvs.Insert("Workers", fmt.Sprint(*o.Workers))
	}
	if o.CustomK8sRegex != "" {
		kvs.Insert("custom_k8s_regex", o.CustomK8sRegex)
	}
	if o.ResourceLabels != nil && len(o.ResourceLabels) > 0 {
		kvs.Insert("resource_labels", utils.ConcatString(o.ResourceLabels, ","))
	}
	return kvs, nil
}
