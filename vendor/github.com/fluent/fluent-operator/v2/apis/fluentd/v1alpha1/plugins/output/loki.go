package output

import (
	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins"
)

// The loki output plugin, allows to ingest your records into a Loki service.
type Loki struct {
	// Loki URL.
	Url *string `json:"url"`
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
	// Optional list of record keys that will be removed from stream labels.
	// This configuration property is for records key only.
	RemoveKeys []string `json:"removeKeys,omitempty"`
	// Format to use when flattening the record to a log line. Valid values are json or key_value.
	// If set to json,  the log line sent to Loki will be the Fluentd record dumped as JSON.
	// If set to key_value, the log line will be each item in the record concatenated together (separated by a single space) in the format.
	// +kubebuilder:validation:Enum:=json;key_value
	LineFormat string `json:"lineFormat,omitempty"`
	// If set to true, it will add all Kubernetes labels to the Stream labels.
	ExtractKubernetesLabels *bool `json:"extractKubernetesLabels,omitempty"`
	// If a record only has 1 key, then just set the log line to the value and discard the key.
	DropSingleKey *bool `json:"dropSingleKey,omitempty"`
	// Whether or not to include the fluentd_thread label when multiple threads are used for flushing
	IncludeThreadLabel *bool `json:"includeThreadLabel,omitempty"`
	// Disable certificate validation
	Insecure *bool `json:"insecure,omitempty"`
	// TlsCaCert defines the CA certificate file for TLS.
	TlsCaCertFile *string `json:"tlsCaCertFile,omitempty"`
	// TlsClientCert defines the client certificate file for TLS.
	TlsClientCertFile *string `json:"tlsClientCertFile,omitempty"`
	// TlsPrivateKey defines the client private key file for TLS.
	TlsPrivateKeyFile *string `json:"tlsPrivateKeyFile,omitempty"`
}
