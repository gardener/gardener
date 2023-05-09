package output

import (
	"fmt"

	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// Elasticsearch is the es output plugin, allows to ingest your records into an Elasticsearch database. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/outputs/elasticsearch**
type Elasticsearch struct {
	// IP address or hostname of the target Elasticsearch instance
	Host string `json:"host,omitempty"`
	// TCP port of the target Elasticsearch instance
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:validation:Maximum:=65535
	Port *int32 `json:"port,omitempty"`
	// Elasticsearch accepts new data on HTTP query path "/_bulk".
	// But it is also possible to serve Elasticsearch behind a reverse proxy on a subpath.
	// This option defines such path on the fluent-bit side.
	// It simply adds a path prefix in the indexing HTTP POST URI.
	Path string `json:"path,omitempty"`
	// Specify the buffer size used to read the response from the Elasticsearch HTTP service.
	// This option is useful for debugging purposes where is required to read full responses,
	// note that response size grows depending of the number of records inserted.
	// To set an unlimited amount of memory set this value to False,
	// otherwise the value must be according to the Unit Size specification.
	// +kubebuilder:validation:Pattern:="^\\d+(k|K|KB|kb|m|M|MB|mb|g|G|GB|gb)?$"
	BufferSize string `json:"bufferSize,omitempty"`
	// Newer versions of Elasticsearch allows setting up filters called pipelines.
	// This option allows defining which pipeline the database should use.
	// For performance reasons is strongly suggested parsing
	// and filtering on Fluent Bit side, avoid pipelines.
	Pipeline string `json:"pipeline,omitempty"`
	// Enable AWS Sigv4 Authentication for Amazon ElasticSearch Service.
	AWSAuth string `json:"awsAuth,omitempty"`
	// Specify the AWS region for Amazon ElasticSearch Service.
	AWSRegion string `json:"awsRegion,omitempty"`
	// Specify the custom sts endpoint to be used with STS API for Amazon ElasticSearch Service.
	AWSSTSEndpoint string `json:"awsSTSEndpoint,omitempty"`
	// AWS IAM Role to assume to put records to your Amazon ES cluster.
	AWSRoleARN string `json:"awsRoleARN,omitempty"`
	// External ID for the AWS IAM Role specified with aws_role_arn.
	AWSExternalID string `json:"awsExternalID,omitempty"`
	// If you are using Elastic's Elasticsearch Service you can specify the cloud_id of the cluster running.
	CloudID string `json:"cloudID,omitempty"`
	// Specify the credentials to use to connect to Elastic's Elasticsearch Service running on Elastic Cloud.
	CloudAuth string `json:"cloudAuth,omitempty"`
	// Optional username credential for Elastic X-Pack access
	HTTPUser *plugins.Secret `json:"httpUser,omitempty"`
	// Password for user defined in HTTP_User
	HTTPPasswd *plugins.Secret `json:"httpPassword,omitempty"`
	// Index name
	Index string `json:"index,omitempty"`
	// Type name
	Type string `json:"type,omitempty"`
	// Enable Logstash format compatibility.
	// This option takes a boolean value: True/False, On/Off
	LogstashFormat *bool `json:"logstashFormat,omitempty"`
	// When Logstash_Format is enabled, the Index name is composed using a prefix and the date,
	// e.g: If Logstash_Prefix is equals to 'mydata' your index will become 'mydata-YYYY.MM.DD'.
	// The last string appended belongs to the date when the data is being generated.
	LogstashPrefix string `json:"logstashPrefix,omitempty"`
	// Time format (based on strftime) to generate the second part of the Index name.
	LogstashDateFormat string `json:"logstashDateFormat,omitempty"`
	// When Logstash_Format is enabled, each record will get a new timestamp field.
	// The Time_Key property defines the name of that field.
	TimeKey string `json:"timeKey,omitempty"`
	// When Logstash_Format is enabled, this property defines the format of the timestamp.
	TimeKeyFormat string `json:"timeKeyFormat,omitempty"`
	// When Logstash_Format is enabled, enabling this property sends nanosecond precision timestamps.
	TimeKeyNanos *bool `json:"timeKeyNanos,omitempty"`
	// When enabled, it append the Tag name to the record.
	IncludeTagKey *bool `json:"includeTagKey,omitempty"`
	// When Include_Tag_Key is enabled, this property defines the key name for the tag.
	TagKey string `json:"tagKey,omitempty"`
	// When enabled, generate _id for outgoing records.
	// This prevents duplicate records when retrying ES.
	GenerateID *bool `json:"generateID,omitempty"`
	// If set, _id will be the value of the key from incoming record and Generate_ID option is ignored.
	IdKey string `json:"idKey,omitempty"`
	// When enabled, replace field name dots with underscore, required by Elasticsearch 2.0-2.3.
	ReplaceDots *bool `json:"replaceDots,omitempty"`
	// When enabled print the elasticsearch API calls to stdout (for diag only)
	TraceOutput *bool `json:"traceOutput,omitempty"`
	// When enabled print the elasticsearch API calls to stdout when elasticsearch returns an error
	TraceError *bool `json:"traceError,omitempty"`
	// Use current time for index generation instead of message record
	CurrentTimeIndex *bool `json:"currentTimeIndex,omitempty"`
	// Prefix keys with this string
	LogstashPrefixKey string `json:"logstashPrefixKey,omitempty"`
	// When enabled, mapping types is removed and Type option is ignored. Types are deprecated in APIs in v7.0. This options is for v7.0 or later.
	SuppressTypeName string `json:"suppressTypeName,omitempty"`
	*plugins.TLS     `json:"tls,omitempty"`
}

// Name implement Section() method
func (_ *Elasticsearch) Name() string {
	return "es"
}

// Params implement Section() method
func (es *Elasticsearch) Params(sl plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()
	if es.Host != "" {
		kvs.Insert("Host", es.Host)
	}
	if es.Port != nil {
		kvs.Insert("Port", fmt.Sprint(*es.Port))
	}
	if es.Path != "" {
		kvs.Insert("Path", es.Path)
	}
	if es.BufferSize != "" {
		kvs.Insert("Buffer_Size", es.BufferSize)
	}
	if es.Pipeline != "" {
		kvs.Insert("Pipeline", es.Pipeline)
	}
	if es.AWSAuth != "" {
		kvs.Insert("AWS_Auth", es.AWSAuth)
	}
	if es.AWSRegion != "" {
		kvs.Insert("AWS_Region", es.AWSRegion)
	}
	if es.AWSSTSEndpoint != "" {
		kvs.Insert("AWS_STS_Endpoint", es.AWSSTSEndpoint)
	}
	if es.AWSRoleARN != "" {
		kvs.Insert("AWS_Role_ARN", es.AWSRoleARN)
	}
	if es.CloudID != "" {
		kvs.Insert("Cloud_ID", es.CloudID)
	}
	if es.CloudAuth != "" {
		kvs.Insert("Cloud_Auth", es.CloudAuth)
	}
	if es.AWSExternalID != "" {
		kvs.Insert("AWS_External_ID", es.AWSExternalID)
	}
	if es.HTTPUser != nil {
		u, err := sl.LoadSecret(*es.HTTPUser)
		if err != nil {
			return nil, err
		}
		kvs.Insert("HTTP_User", u)
	}
	if es.HTTPPasswd != nil {
		pwd, err := sl.LoadSecret(*es.HTTPPasswd)
		if err != nil {
			return nil, err
		}
		kvs.Insert("HTTP_Passwd", pwd)
	}
	if es.Index != "" {
		kvs.Insert("Index", es.Index)
	}
	if es.Type != "" {
		kvs.Insert("Type", es.Type)
	}
	if es.LogstashFormat != nil {
		kvs.Insert("Logstash_Format", fmt.Sprint(*es.LogstashFormat))
	}
	if es.LogstashPrefix != "" {
		kvs.Insert("Logstash_Prefix", es.LogstashPrefix)
	}
	if es.LogstashDateFormat != "" {
		kvs.Insert("Logstash_DateFormat", es.LogstashDateFormat)
	}
	if es.TimeKey != "" {
		kvs.Insert("Time_Key", es.TimeKey)
	}
	if es.TimeKeyFormat != "" {
		kvs.Insert("Time_Key_Format", es.TimeKeyFormat)
	}
	if es.TimeKeyNanos != nil {
		kvs.Insert("Time_Key_Nanos", fmt.Sprint(*es.TimeKeyNanos))
	}
	if es.IncludeTagKey != nil {
		kvs.Insert("Include_Tag_Key", fmt.Sprint(*es.IncludeTagKey))
	}
	if es.TagKey != "" {
		kvs.Insert("Tag_Key", es.TagKey)
	}
	if es.GenerateID != nil {
		kvs.Insert("Generate_ID", fmt.Sprint(*es.GenerateID))
	}
	if es.IdKey != "" {
		kvs.Insert("ID_KEY", es.IdKey)
	}
	if es.ReplaceDots != nil {
		kvs.Insert("Replace_Dots", fmt.Sprint(*es.ReplaceDots))
	}
	if es.TraceOutput != nil {
		kvs.Insert("Trace_Output", fmt.Sprint(*es.TraceOutput))
	}
	if es.TraceError != nil {
		kvs.Insert("Trace_Error", fmt.Sprint(*es.TraceError))
	}
	if es.CurrentTimeIndex != nil {
		kvs.Insert("Current_Time_Index", fmt.Sprint(*es.CurrentTimeIndex))
	}
	if es.LogstashPrefixKey != "" {
		kvs.Insert("Logstash_Prefix_Key", es.LogstashPrefixKey)
	}
	if es.SuppressTypeName != "" {
		kvs.Insert("Suppress_Type_Name", es.SuppressTypeName)
	}
	if es.TLS != nil {
		tls, err := es.TLS.Params(sl)
		if err != nil {
			return nil, err
		}
		kvs.Merge(tls)
	}
	return kvs, nil
}
