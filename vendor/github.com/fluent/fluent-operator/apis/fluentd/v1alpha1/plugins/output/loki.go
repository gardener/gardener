package output

import "github.com/fluent/fluent-operator/apis/fluentd/v1alpha1/plugins"

// The loki output plugin, allows to ingest your records into a Loki service.
type Loki struct {
	// Loki hostname or IP address.
	Host *string `json:"host"`
	// Loki TCP port
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=65535
	Port *int32 `json:"port,omitempty"`
	// Set HTTP basic authentication user name.
	HTTPUser *plugins.Secret `json:"httpUser,omitempty"`
	// Password for user defined in HTTP_User
	// Set HTTP basic authentication password
	HTTPPasswd *plugins.Secret `json:"httpPassword,omitempty"`
	// Tenant ID used by default to push logs to Loki.
	// If omitted or empty it assumes Loki is running in single-tenant mode and no X-Scope-OrgID header is sent.
	TenantID *plugins.Secret `json:"tenantID,omitempty"`
	// Stream labels for API request. It can be multiple comma separated of strings specifying  key=value pairs.
	// In addition to fixed parameters, it also allows to add custom record keys (similar to label_keys property).
	Labels []string `json:"labels,omitempty"`
	// Optional list of record keys that will be placed as stream labels.
	// This configuration property is for records key only.
	LabelKeys []string `json:"labelKeys,omitempty"`
	// Format to use when flattening the record to a log line. Valid values are json or key_value.
	// If set to json,  the log line sent to Loki will be the Fluentd record dumped as JSON.
	// If set to key_value, the log line will be each item in the record concatenated together (separated by a single space) in the format.
	// +kubebuilder:validation:Enum:=json;key_value
	LineFormat string `json:"lineFormat,omitempty"`
	// If set to true, it will add all Kubernetes labels to the Stream labels.
	// +kubebuilder:validation:Enum:=on;off
	AutoKubernetesLabels string `json:"autoKubernetesLabels,omitempty"`
	*plugins.TLS         `json:"tls,omitempty"`
}
