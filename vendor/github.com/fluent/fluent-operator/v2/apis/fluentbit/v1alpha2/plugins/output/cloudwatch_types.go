package output

import (
	"fmt"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// CloudWatch is the AWS CloudWatch output plugin, allows you to ingest your records into AWS CloudWatch. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/outputs/cloudwatch**
type CloudWatch struct {
	// AWS Region
	Region string `json:"region"`
	// Name of Cloudwatch Log Group to send log records to
	LogGroupName string `json:"logGroupName,omitempty"`
	// Template for Log Group name, overrides LogGroupName if set.
	LogGroupTemplate string `json:"logGroupTemplate,omitempty"`
	// The name of the CloudWatch Log Stream to send log records to
	LogStreamName string `json:"logStreamName,omitempty"`
	// Prefix for the Log Stream name. Not compatible with LogStreamName setting
	LogStreamPrefix string `json:"logStreamPrefix,omitempty"`
	// Template for Log Stream name. Overrides LogStreamPrefix and LogStreamName if set.
	LogStreamTemplate string `json:"logStreamTemplate,omitempty"`
	// If set, only the value of the key will be sent to CloudWatch
	LogKey string `json:"logKey,omitempty"`
	// Optional parameter to tell CloudWatch the format of the data
	LogFormat string `json:"logFormat,omitempty"`
	// Role ARN to use for cross-account access
	RoleArn string `json:"roleArn,omitempty"`
	// Automatically create the log group. Defaults to False.
	AutoCreateGroup *bool `json:"autoCreateGroup,omitempty"`
	// Number of days logs are retained for
	// +kubebuilder:validation:Enum:=1;3;5;7;14;30;60;90;120;150;180;365;400;545;731;1827;3653
	LogRetentionDays *int32 `json:"logRetentionDays,omitempty"`
	// Custom endpoint for CloudWatch logs API
	Endpoint string `json:"endpoint,omitempty"`
	// Optional string to represent the CloudWatch namespace.
	MetricNamespace string `json:"metricNamespace,omitempty"`
	// Optional lists of lists for dimension keys to be added to all metrics. Use comma separated strings
	// for one list of dimensions and semicolon separated strings for list of lists dimensions.
	MetricDimensions string `json:"metricDimensions,omitempty"`
	// Specify a custom STS endpoint for the AWS STS API
	StsEndpoint string `json:"stsEndpoint,omitempty"`
	// Automatically retry failed requests to CloudWatch once. Defaults to True.
	AutoRetryRequests *bool `json:"autoRetryRequests,omitempty"`
	// Specify an external ID for the STS API.
	ExternalID string `json:"externalID,omitempty"`
}

// Name implement Section() method
func (_ *CloudWatch) Name() string {
	return "cloudwatch_logs"
}

// Params implement Section() method
func (o *CloudWatch) Params(sl plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	if o.Region != "" {
		kvs.Insert("region", o.Region)
	}
	if o.LogGroupName != "" {
		kvs.Insert("log_group_name", o.LogGroupName)
	}
	if o.LogGroupTemplate != "" {
		kvs.Insert("log_group_template", o.LogGroupTemplate)
	}
	if o.LogStreamName != "" {
		kvs.Insert("log_stream_name", o.LogStreamName)
	}
	if o.LogStreamPrefix != "" {
		kvs.Insert("log_stream_prefix", o.LogStreamPrefix)
	}
	if o.LogStreamTemplate != "" {
		kvs.Insert("log_stream_template", o.LogStreamTemplate)
	}
	if o.LogKey != "" {
		kvs.Insert("log_key", o.LogKey)
	}
	if o.LogFormat != "" {
		kvs.Insert("log_format", o.LogFormat)
	}
	if o.AutoCreateGroup != nil {
		kvs.Insert("auto_create_group", fmt.Sprint(*o.AutoCreateGroup))
	}
	if o.LogRetentionDays != nil {
		kvs.Insert("log_retention_days", fmt.Sprint(*o.LogRetentionDays))
	}
	if o.RoleArn != "" {
		kvs.Insert("role_arn", o.RoleArn)
	}
	if o.Endpoint != "" {
		kvs.Insert("endpoint", o.Endpoint)
	}
	if o.MetricNamespace != "" {
		kvs.Insert("metric_namespace", o.MetricNamespace)
	}
	if o.MetricDimensions != "" {
		kvs.Insert("metric_dimensions", o.MetricDimensions)
	}
	if o.StsEndpoint != "" {
		kvs.Insert("sts_endpoint", o.StsEndpoint)
	}
	if o.AutoRetryRequests != nil {
		kvs.Insert("auto_retry_requests", fmt.Sprint(*o.AutoRetryRequests))
	}
	if o.ExternalID != "" {
		kvs.Insert("external_id", o.ExternalID)
	}
	return kvs, nil
}
