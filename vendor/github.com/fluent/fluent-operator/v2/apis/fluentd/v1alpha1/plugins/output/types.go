package output

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins/common"
	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins/custom"
	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins/params"
	"github.com/fluent/fluent-operator/v2/pkg/utils"
	"strconv"
)

// OutputCommon defines the common parameters for output plugin
type OutputCommon struct {
	Id *string `json:"-"`
	// The @log_level parameter specifies the plugin-specific logging level
	LogLevel *string `json:"logLevel,omitempty"`
	// The @label parameter is to route the events to <label> sections
	Label *string `json:"-"`
	// Which tag to be matched.
	Tag *string `json:"-"`
}

// Output defines all available output plugins and their parameters
type Output struct {
	OutputCommon `json:",inline,omitempty"`
	// match setions
	common.BufferSection `json:",inline,omitempty"`
	// out_forward plugin
	Forward *Forward `json:"forward,omitempty"`
	// out_http plugin
	Http *Http `json:"http,omitempty"`
	// out_es plugin
	Elasticsearch *Elasticsearch `json:"elasticsearch,omitempty"`
	// out_opensearch plugin
	Opensearch *Opensearch `json:"opensearch,omitempty"`
	// out_kafka plugin
	Kafka *Kafka2 `json:"kafka,omitempty"`
	// out_s3 plugin
	S3 *S3 `json:"s3,omitempty"`
	// out_stdout plugin
	Stdout *Stdout `json:"stdout,omitempty"`
	// out_loki plugin
	Loki *Loki `json:"loki,omitempty"`
	// Custom plugin type
	CustomPlugin *custom.CustomPlugin `json:"customPlugin,omitempty"`
	// out_cloudwatch plugin
	CloudWatch *CloudWatch `json:"cloudWatch,omitempty"`
}

// DeepCopyInto implements the DeepCopyInto interface.
func (in *Output) DeepCopyInto(out *Output) {
	bytes, err := json.Marshal(*in)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(bytes, &out)
	if err != nil {
		panic(err)
	}
}

func (o *Output) Name() string {
	return "match"
}

func (o *Output) Params(loader plugins.SecretLoader) (*params.PluginStore, error) {
	ps := params.NewPluginStore(o.Name())
	childs := make([]*params.PluginStore, 0)

	ps.InsertPairs("@id", fmt.Sprint(*o.Id))

	if o.LogLevel != nil {
		ps.InsertPairs("@log_level", fmt.Sprint(*o.LogLevel))
	}

	if o.Label != nil {
		ps.InsertPairs("@label", fmt.Sprint(*o.Label))
	}

	if o.Tag != nil {
		ps.InsertPairs("tag", fmt.Sprint(*o.Tag))
	}

	if o.BufferSection.Buffer != nil {
		child, _ := o.BufferSection.Buffer.Params(loader)
		childs = append(childs, child)
	}
	if o.BufferSection.Inject != nil {
		child, _ := o.BufferSection.Inject.Params(loader)
		childs = append(childs, child)
	}
	if o.BufferSection.Format != nil {
		child, _ := o.BufferSection.Format.Params(loader)
		childs = append(childs, child)
	}

	ps.InsertChilds(childs...)

	if o.Forward != nil {
		ps.InsertType(string(params.ForwardOutputType))
		return o.forwardPlugin(ps, loader), nil
	}

	if o.Http != nil {
		ps.InsertType(string(params.HttpOutputType))
		return o.httpPlugin(ps, loader), nil
	}

	if o.Kafka != nil {
		ps.InsertType(string(params.KafkaOutputType))

		// kafka format section can not be empty
		if o.Format == nil {
			o.Format = &common.Format{
				FormatCommon: common.FormatCommon{
					Type: &params.DefaultFormatType,
				},
			}
			child, _ := o.BufferSection.Format.Params(loader)
			ps.InsertChilds(child)
		}
		return o.kafka2Plugin(ps, loader), nil
	}

	if o.Elasticsearch != nil {
		ps.InsertType(string(params.ElasticsearchOutputType))
		return o.elasticsearchPlugin(ps, loader)
	}

	if o.Opensearch != nil {
		ps.InsertType(string(params.OpensearchOutputType))
		return o.opensearchPlugin(ps, loader)
	}

	if o.S3 != nil {
		ps.InsertType(string(params.S3OutputType))
		return o.s3Plugin(ps, loader), nil
	}

	if o.Loki != nil {
		ps.InsertType(string(params.LokiOutputType))
		return o.lokiPlugin(ps, loader), nil
	}

	if o.Stdout != nil {
		ps.InsertType(string(params.StdOutputType))
		return o.stdoutPlugin(ps, loader), nil
	}

	if o.CloudWatch != nil {
		ps.InsertType(string(params.CloudWatchOutputType))
		return o.cloudWatchPlugin(ps, loader), nil
	}
	return o.customOutput(ps, loader), nil

}

func (o *Output) forwardPlugin(parent *params.PluginStore, loader plugins.SecretLoader) *params.PluginStore {
	childs := make([]*params.PluginStore, 0)

	if len(o.Forward.Servers) > 0 {
		for _, s := range o.Forward.Servers {
			child, _ := s.Params(loader)
			childs = append(childs, child)
		}
	}

	if o.Forward.ServiceDiscovery != nil {
		child, _ := o.Forward.ServiceDiscovery.Params(loader)
		childs = append(childs, child)
	}

	if o.Forward.Security != nil {
		child, _ := o.Forward.Security.Params(loader)
		childs = append(childs, child)
	}

	parent.InsertChilds(childs...)

	if o.Forward.RequireAckResponse != nil {
		parent.InsertPairs("require_ack_response", fmt.Sprint(*o.Forward.RequireAckResponse))
	}

	if o.Forward.SendTimeout != nil {
		parent.InsertPairs("send_timeout", fmt.Sprint(*o.Forward.SendTimeout))
	}

	if o.Forward.ConnectTimeout != nil {
		parent.InsertPairs("connect_timeout", fmt.Sprint(*o.Forward.ConnectTimeout))
	}

	if o.Forward.RecoverWait != nil {
		parent.InsertPairs("recover_wait", fmt.Sprint(*o.Forward.RecoverWait))
	}

	if o.Forward.AckResponseTimeout != nil {
		parent.InsertPairs("heartbeat_type", fmt.Sprint(*o.Forward.HeartbeatType))
	}

	if o.Forward.HeartbeatInterval != nil {
		parent.InsertPairs("heartbeat_interval", fmt.Sprint(*o.Forward.HeartbeatInterval))
	}

	if o.Forward.PhiFailureDetector != nil {
		parent.InsertPairs("phi_failure_detector", fmt.Sprint(*o.Forward.PhiFailureDetector))
	}

	if o.Forward.PhiThreshold != nil {
		parent.InsertPairs("phi_threshold", fmt.Sprint(*o.Forward.PhiThreshold))
	}

	if o.Forward.HardTimeout != nil {
		parent.InsertPairs("hard_timeout", fmt.Sprint(*o.Forward.HardTimeout))
	}

	if o.Forward.ExpireDnsCache != nil {
		parent.InsertPairs("expire_dns_cache", fmt.Sprint(*o.Forward.ExpireDnsCache))
	}

	if o.Forward.DnsRoundRobin != nil {
		parent.InsertPairs("dns_round_robin", fmt.Sprint(*o.Forward.DnsRoundRobin))
	}

	if o.Forward.IgnoreNetworkErrorsAtStartup != nil {
		parent.InsertPairs("ignore_network_errors_at_startup", fmt.Sprint(*o.Forward.IgnoreNetworkErrorsAtStartup))
	}

	if o.Forward.TlsVersion != nil {
		parent.InsertPairs("tls_version", fmt.Sprint(*o.Forward.TlsVersion))
	}

	if o.Forward.TlsCiphers != nil {
		parent.InsertPairs("tls_ciphers", fmt.Sprint(*o.Forward.TlsCiphers))
	}

	if o.Forward.TlsInsecureMode != nil {
		parent.InsertPairs("tls_insecure_mode", fmt.Sprint(*o.Forward.TlsInsecureMode))
	}

	if o.Forward.TlsAllowSelfSignedCert != nil {
		parent.InsertPairs("tls_allow_self_signed_cert", fmt.Sprint(*o.Forward.TlsAllowSelfSignedCert))
	}

	if o.Forward.TlsVerifyHostname != nil {
		parent.InsertPairs("tls_verify_hostname", fmt.Sprint(*o.Forward.TlsVerifyHostname))
	}

	if o.Forward.TlsCertPath != nil {
		parent.InsertPairs("tls_cert_path", fmt.Sprint(*o.Forward.TlsCertPath))
	}
	if o.Forward.TlsClientCertPath != nil {
		parent.InsertPairs("tls_client_cert_path", fmt.Sprint(*o.Forward.TlsClientCertPath))
	}
	if o.Forward.TlsClientPrivateKeyPath != nil {
		parent.InsertPairs("tls_client_private_key_path", fmt.Sprint(*o.Forward.TlsClientPrivateKeyPath))
	}
	if o.Forward.TlsClientPrivateKeyPassphrase != nil {
		parent.InsertPairs("tls_client_private_key_passphrase", fmt.Sprint(*o.Forward.TlsClientPrivateKeyPassphrase))
	}
	if o.Forward.TlsCertThumbprint != nil {
		parent.InsertPairs("tls_cert_thumbprint", fmt.Sprint(*o.Forward.TlsCertThumbprint))
	}
	if o.Forward.TlsCertLogicalStoreName != nil {
		parent.InsertPairs("tls_cert_logical_storeName", fmt.Sprint(*o.Forward.TlsCertLogicalStoreName))
	}
	if o.Forward.TlsCertUseEnterpriseStore != nil {
		parent.InsertPairs("tls_cert_use_enterprise_store", fmt.Sprint(*o.Forward.TlsCertUseEnterpriseStore))
	}
	if o.Forward.Keepalive != nil {
		parent.InsertPairs("keepalive", fmt.Sprint(*o.Forward.Keepalive))
	}
	if o.Forward.KeepaliveTimeout != nil {
		parent.InsertPairs("keepalive_timeout", fmt.Sprint(*o.Forward.KeepaliveTimeout))
	}
	if o.Forward.VerifyConnectionAtStartup != nil {
		parent.InsertPairs("verify_connection_at_startup", fmt.Sprint(*o.Forward.VerifyConnectionAtStartup))
	}

	return parent
}

func (o *Output) httpPlugin(parent *params.PluginStore, loader plugins.SecretLoader) *params.PluginStore {
	if o.Http.Auth != nil {
		child, _ := o.Http.Params(loader)
		parent.InsertChilds(child)
	}

	if o.Http.Endpoint != nil {
		parent.InsertPairs("endpoint", fmt.Sprint(*o.Http.Endpoint))
	}

	if o.Http.HttpMethod != nil {
		parent.InsertPairs("http_method", fmt.Sprint(*o.Http.HttpMethod))
	}

	if o.Http.Proxy != nil {
		parent.InsertPairs("proxy", fmt.Sprint(*o.Http.Proxy))
	}

	if o.Http.ContentType != nil {
		parent.InsertPairs("content_type", fmt.Sprint(*o.Http.ContentType))
	}

	if o.Http.JsonArray != nil {
		parent.InsertPairs("json_array", fmt.Sprint(*o.Http.JsonArray))
	}

	if o.Http.Headers != nil {
		parent.InsertPairs("headers", fmt.Sprint(*o.Http.Headers))
	}

	if o.Http.HeadersFromPlaceholders != nil {
		parent.InsertPairs("headers_from_placeholders", fmt.Sprint(*o.Http.HeadersFromPlaceholders))
	}

	if o.Http.OpenTimeout != nil {
		parent.InsertPairs("open_timeout", fmt.Sprint(*o.Http.OpenTimeout))
	}

	if o.Http.ReadTimeout != nil {
		parent.InsertPairs("read_timeout", fmt.Sprint(*o.Http.ReadTimeout))
	}

	if o.Http.SslTimeout != nil {
		parent.InsertPairs("ssl_timeout", fmt.Sprint(*o.Http.SslTimeout))
	}

	if o.Http.TlsCaCertPath != nil {
		parent.InsertPairs("tls_ca_cert_path", fmt.Sprint(*o.Http.TlsCaCertPath))
	}

	if o.Http.TlsClientCertPath != nil {
		parent.InsertPairs("tls_client_cert_path", fmt.Sprint(*o.Http.TlsClientCertPath))
	}

	if o.Http.TlsPrivateKeyPath != nil {
		parent.InsertPairs("tls_private_key_path", fmt.Sprint(*o.Http.TlsPrivateKeyPath))
	}

	if o.Http.TlsPrivateKeyPassphrase != nil {
		parent.InsertPairs("tls_private_key_passphrase", fmt.Sprint(*o.Http.TlsPrivateKeyPassphrase))
	}

	if o.Http.TlsVerifyMode != nil {
		parent.InsertPairs("tls_verify_mode", fmt.Sprint(*o.Http.TlsVerifyMode))
	}

	if o.Http.TlsVersion != nil {
		parent.InsertPairs("tls_version", fmt.Sprint(*o.Http.TlsVersion))
	}

	if o.Http.TlsCiphers != nil {
		parent.InsertPairs("tls_ciphers", fmt.Sprint(*o.Http.TlsCiphers))
	}

	if o.Http.ErrorResponseAsUnrecoverable != nil {
		parent.InsertPairs("error_response_as_unrecoverable", fmt.Sprint(*o.Http.ErrorResponseAsUnrecoverable))
	}

	if o.Http.RetryableResponseCodes != nil {
		parent.InsertPairs("retryable_response_codes", fmt.Sprint(*o.Http.RetryableResponseCodes))
	}

	return parent
}

func (o *Output) elasticsearchPlugin(parent *params.PluginStore, loader plugins.SecretLoader) (*params.PluginStore, error) {
	if o.Elasticsearch.Host != nil {
		parent.InsertPairs("host", fmt.Sprint(*o.Elasticsearch.Host))
	}

	if o.Elasticsearch.Port != nil {
		parent.InsertPairs("port", fmt.Sprint(*o.Elasticsearch.Port))
	}

	if o.Elasticsearch.Hosts != nil {
		parent.InsertPairs("hosts", fmt.Sprint(*o.Elasticsearch.Hosts))
	}

	if o.Elasticsearch.User != nil {
		user, err := loader.LoadSecret(*o.Elasticsearch.User)
		if err != nil {
			return nil, err
		}
		parent.InsertPairs("user", user)
	}

	if o.Elasticsearch.Password != nil {
		pwd, err := loader.LoadSecret(*o.Elasticsearch.Password)
		if err != nil {
			return nil, err
		}
		parent.InsertPairs("password", pwd)
	}

	if o.Elasticsearch.Scheme != nil {
		parent.InsertPairs("scheme", fmt.Sprint(*o.Elasticsearch.Scheme))
	}

	if o.Elasticsearch.Path != nil {
		parent.InsertPairs("path", fmt.Sprint(*o.Elasticsearch.Path))
	}

	if o.Elasticsearch.IndexName != nil {
		parent.InsertPairs("index_name", fmt.Sprint(*o.Elasticsearch.IndexName))
	}

	if o.Elasticsearch.LogstashFormat != nil {
		parent.InsertPairs("logstash_format", fmt.Sprint(*o.Elasticsearch.LogstashFormat))
	}

	if o.Elasticsearch.LogstashPrefix != nil {
		parent.InsertPairs("logstash_prefix", fmt.Sprint(*o.Elasticsearch.LogstashPrefix))
	}

	return parent, nil
}

func (o *Output) opensearchPlugin(parent *params.PluginStore, loader plugins.SecretLoader) (*params.PluginStore, error) {
	if o.Opensearch.Host != nil {
		parent.InsertPairs("host", fmt.Sprint(*o.Opensearch.Host))
	}

	if o.Opensearch.Port != nil {
		parent.InsertPairs("port", fmt.Sprint(*o.Opensearch.Port))
	}

	if o.Opensearch.Hosts != nil {
		parent.InsertPairs("hosts", fmt.Sprint(*o.Opensearch.Hosts))
	}

	if o.Opensearch.User != nil {
		user, err := loader.LoadSecret(*o.Opensearch.User)
		if err != nil {
			return nil, err
		}
		parent.InsertPairs("user", user)
	}

	if o.Opensearch.Password != nil {
		pwd, err := loader.LoadSecret(*o.Opensearch.Password)
		if err != nil {
			return nil, err
		}
		parent.InsertPairs("password", pwd)
	}

	if o.Opensearch.Scheme != nil {
		parent.InsertPairs("scheme", fmt.Sprint(*o.Opensearch.Scheme))
	}

	if o.Opensearch.Path != nil {
		parent.InsertPairs("path", fmt.Sprint(*o.Opensearch.Path))
	}

	if o.Opensearch.IndexName != nil {
		parent.InsertPairs("index_name", fmt.Sprint(*o.Opensearch.IndexName))
	}

	if o.Opensearch.LogstashFormat != nil {
		parent.InsertPairs("logstash_format", fmt.Sprint(*o.Opensearch.LogstashFormat))
	}

	if o.Opensearch.LogstashPrefix != nil {
		parent.InsertPairs("logstash_prefix", fmt.Sprint(*o.Opensearch.LogstashPrefix))
	}

	return parent, nil
}

func (o *Output) kafka2Plugin(parent *params.PluginStore, loader plugins.SecretLoader) *params.PluginStore {
	if o.Kafka.Brokers != nil {
		parent.InsertPairs("brokers", fmt.Sprint(*o.Kafka.Brokers))
	}
	if o.Kafka.TopicKey != nil {
		parent.InsertPairs("topic_key", fmt.Sprint(*o.Kafka.TopicKey))
	}
	if o.Kafka.DefaultTopic != nil {
		parent.InsertPairs("default_topic", fmt.Sprint(*o.Kafka.DefaultTopic))
	}
	if o.Kafka.UseEventTime != nil {
		parent.InsertPairs("use_event_time", fmt.Sprint(*o.Kafka.UseEventTime))
	}
	if o.Kafka.RequiredAcks != nil {
		parent.InsertPairs("required_acks", fmt.Sprint(*o.Kafka.RequiredAcks))
	}
	if o.Kafka.CompressionCodec != nil {
		parent.InsertPairs("compression_codec", fmt.Sprint(*o.Kafka.CompressionCodec))
	}

	return parent
}

func (o *Output) s3Plugin(parent *params.PluginStore, loader plugins.SecretLoader) *params.PluginStore {
	if o.S3.AwsKeyId != nil {
		parent.InsertPairs("aws_key_id", fmt.Sprint(*o.S3.AwsKeyId))
	}
	if o.S3.AwsSecKey != nil {
		parent.InsertPairs("aws_sec_key", fmt.Sprint(*o.S3.AwsSecKey))
	}
	if o.S3.S3Bucket != nil {
		parent.InsertPairs("s3_bucket", fmt.Sprint(*o.S3.S3Bucket))
	}
	if o.S3.Path != nil {
		parent.InsertPairs("path", fmt.Sprint(*o.S3.Path))
	}
	if o.S3.S3ObjectKeyFormat != nil {
		parent.InsertPairs("s3_object_key_format", fmt.Sprint(*o.S3.S3ObjectKeyFormat))
	}
	if o.S3.StoreAs != nil {
		parent.InsertPairs("store_as", fmt.Sprint(*o.S3.StoreAs))
	}
	if o.S3.ProxyUri != nil {
		parent.InsertPairs("proxy_uri", fmt.Sprint(*o.S3.ProxyUri))
	}
	if o.S3.SslVerifyPeer != nil {
		parent.InsertPairs("ssl_verify_peer", fmt.Sprint(*o.S3.SslVerifyPeer))
	}
	return parent
}

func (o *Output) lokiPlugin(parent *params.PluginStore, loader plugins.SecretLoader) *params.PluginStore {
	if o.Loki.Url != nil {
		parent.InsertPairs("url", fmt.Sprint(*o.Loki.Url))
	}
	if o.Loki.HTTPUser != nil {
		u, err := loader.LoadSecret(*o.Loki.HTTPUser)
		if err != nil {
			return nil
		}
		parent.InsertPairs("username", u)
	}
	if o.Loki.HTTPPasswd != nil {
		passwd, err := loader.LoadSecret(*o.Loki.HTTPPasswd)
		if err != nil {
			return nil
		}
		parent.InsertPairs("password", passwd)
	}
	if o.Loki.TenantID != nil {
		id, err := loader.LoadSecret(*o.Loki.TenantID)
		if err != nil {
			return nil
		}
		parent.InsertPairs("tenant", id)
	}
	if o.Loki.Labels != nil && len(o.Loki.Labels) > 0 {
		labels := make(map[string]string)
		for _, l := range o.Loki.Labels {
			key, value, found := strings.Cut(l, "=")
			if !found {
				continue
			}
			labels[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
		if len(labels) > 0 {
			jsonStr, err := json.Marshal(labels)
			if err != nil {
				fmt.Printf("Error: %s", err.Error())
			} else {
				parent.InsertPairs("extra_labels", string(jsonStr))
			}
		}
	}
	if o.Loki.RemoveKeys != nil && len(o.Loki.RemoveKeys) > 0 {
		parent.InsertPairs("remove_keys", utils.ConcatString(o.Loki.RemoveKeys, ","))
	}
	if o.Loki.LabelKeys != nil && len(o.Loki.LabelKeys) > 0 {
		ps := params.NewPluginStore("label")
		for _, n := range o.Loki.LabelKeys {
			ps.InsertPairs(n, n)
		}
		parent.InsertChilds(ps)
	}
	if o.Loki.LineFormat != "" {
		parent.InsertPairs("line_format", o.Loki.LineFormat)
	}
	if o.Loki.ExtractKubernetesLabels != nil {
		parent.InsertPairs("extract_kubernetes_labels", fmt.Sprint(*o.Loki.ExtractKubernetesLabels))
	}
	if o.Loki.DropSingleKey != nil {
		parent.InsertPairs("drop_single_key", fmt.Sprint(*o.Loki.DropSingleKey))
	}
	if o.Loki.IncludeThreadLabel != nil {
		parent.InsertPairs("include_thread_label", fmt.Sprint(*o.Loki.IncludeThreadLabel))
	}
	if o.Loki.Insecure != nil {
		parent.InsertPairs("insecure_tls", fmt.Sprint(*o.Loki.Insecure))
	}
	if o.Loki.TlsCaCertFile != nil {
		parent.InsertPairs("ca_cert", fmt.Sprint(*o.Loki.TlsCaCertFile))
	}
	if o.Loki.TlsClientCertFile != nil {
		parent.InsertPairs("cert", fmt.Sprint(*o.Loki.TlsClientCertFile))
	}
	if o.Loki.TlsPrivateKeyFile != nil {
		parent.InsertPairs("key", fmt.Sprint(*o.Loki.TlsPrivateKeyFile))
	}
	return parent
}

func (o *Output) cloudWatchPlugin(parent *params.PluginStore, sl plugins.SecretLoader) *params.PluginStore {
	childs := make([]*params.PluginStore, 0)

	if o.CloudWatch.AutoCreateStream != nil {
		parent.InsertPairs("auto_create_stream", strconv.FormatBool(*o.CloudWatch.AutoCreateStream))
	}
	if o.CloudWatch.AwsKeyId != nil {
		value, err := sl.LoadSecret(*o.CloudWatch.AwsKeyId)
		if err != nil {
			return nil
		}
		parent.InsertPairs("aws_key_id", value)
	}
	if o.CloudWatch.AwsSecKey != nil {
		value, err := sl.LoadSecret(*o.CloudWatch.AwsSecKey)
		if err != nil {
			return nil
		}
		parent.InsertPairs("aws_sec_key", value)
	}
	if o.CloudWatch.AwsUseSts != nil {
		parent.InsertPairs("aws_use_sts", strconv.FormatBool(*o.CloudWatch.AwsUseSts))
	}
	if o.CloudWatch.AwsStsRoleARN != nil && *o.CloudWatch.AwsStsRoleARN != "" {
		parent.InsertPairs("aws_sts_role_arn", *o.CloudWatch.AwsStsRoleARN)
	}
	if o.CloudWatch.AwsStsSessionName != nil && *o.CloudWatch.AwsStsSessionName != "" {
		parent.InsertPairs("aws_sts_session_name", *o.CloudWatch.AwsStsSessionName)
	}
	if o.CloudWatch.AwsStsExternalId != nil && *o.CloudWatch.AwsStsExternalId != "" {
		parent.InsertPairs("aws_sts_external_id", *o.CloudWatch.AwsStsExternalId)
	}
	if o.CloudWatch.AwsStsPolicy != nil && *o.CloudWatch.AwsStsPolicy != "" {
		parent.InsertPairs("aws_sts_policy", *o.CloudWatch.AwsStsPolicy)
	}
	if o.CloudWatch.AwsStsDurationSeconds != nil && *o.CloudWatch.AwsStsDurationSeconds != "" {
		parent.InsertPairs("aws_sts_duration_seconds", *o.CloudWatch.AwsStsDurationSeconds)
	}
	if o.CloudWatch.AwsStsEndpointUrl != nil && *o.CloudWatch.AwsStsEndpointUrl != "" {
		parent.InsertPairs("aws_sts_endpoint_url", *o.CloudWatch.AwsStsEndpointUrl)
	}
	if o.CloudWatch.AwsEcsAuthentication != nil {
		parent.InsertPairs("aws_ecs_authentication", strconv.FormatBool(*o.CloudWatch.AwsEcsAuthentication))
	}
	if o.CloudWatch.Concurrency != nil {
		parent.InsertPairs("concurrency", strconv.FormatInt(int64(*o.CloudWatch.Concurrency), 10))
	}
	if o.CloudWatch.Endpoint != nil && *o.CloudWatch.Endpoint != "" {
		parent.InsertPairs("endpoint", *o.CloudWatch.Endpoint)
	}
	if o.CloudWatch.SslVerifyPeer != nil {
		parent.InsertPairs("ssl_verify_peer", strconv.FormatBool(*o.CloudWatch.SslVerifyPeer))
	}
	if o.CloudWatch.HttpProxy != nil && *o.CloudWatch.HttpProxy != "" {
		parent.InsertPairs("http_proxy", *o.CloudWatch.HttpProxy)
	}
	if o.CloudWatch.IncludeTimeKey != nil {
		parent.InsertPairs("include_time_key", strconv.FormatBool(*o.CloudWatch.IncludeTimeKey))
	}
	if o.CloudWatch.JsonHandler != nil && *o.CloudWatch.JsonHandler != "" {
		parent.InsertPairs("json_handler", *o.CloudWatch.JsonHandler)
	}
	if o.CloudWatch.Localtime != nil {
		parent.InsertPairs("localtime", strconv.FormatBool(*o.CloudWatch.Localtime))
	}
	if o.CloudWatch.LogGroupAwsTags != nil && *o.CloudWatch.LogGroupAwsTags != "" {
		parent.InsertPairs("log_group_aws_tags", *o.CloudWatch.LogGroupAwsTags)
	}
	if o.CloudWatch.LogGroupAwsTagsKey != nil && *o.CloudWatch.LogGroupAwsTagsKey != "" {
		parent.InsertPairs("log_group_aws_tags_key", *o.CloudWatch.LogGroupAwsTagsKey)
	}
	if o.CloudWatch.LogGroupName != nil && *o.CloudWatch.LogGroupName != "" {
		parent.InsertPairs("log_group_name", *o.CloudWatch.LogGroupName)
	}
	if o.CloudWatch.LogGroupNameKey != nil && *o.CloudWatch.LogGroupNameKey != "" {
		parent.InsertPairs("log_group_name_key", *o.CloudWatch.LogGroupNameKey)
	}
	if o.CloudWatch.LogRejectedRequest != nil && *o.CloudWatch.LogRejectedRequest != "" {
		parent.InsertPairs("log_rejected_request", *o.CloudWatch.LogRejectedRequest)
	}
	if o.CloudWatch.LogStreamName != nil && *o.CloudWatch.LogStreamName != "" {
		parent.InsertPairs("log_stream_name", *o.CloudWatch.LogStreamName)
	}
	if o.CloudWatch.LogStreamNameKey != nil && *o.CloudWatch.LogStreamNameKey != "" {
		parent.InsertPairs("log_stream_name_key", *o.CloudWatch.LogStreamNameKey)
	}
	if o.CloudWatch.MaxEventsPerBatch != nil && *o.CloudWatch.MaxEventsPerBatch != "" {
		parent.InsertPairs("max_events_per_batch", *o.CloudWatch.MaxEventsPerBatch)
	}
	if o.CloudWatch.MaxMessageLength != nil && *o.CloudWatch.MaxMessageLength != "" {
		parent.InsertPairs("max_message_length", *o.CloudWatch.MaxMessageLength)
	}
	if o.CloudWatch.MessageKeys != nil && *o.CloudWatch.MessageKeys != "" {
		parent.InsertPairs("message_keys", *o.CloudWatch.MessageKeys)
	}
	if o.CloudWatch.PutLogEventsDisableRetryLimit != nil {
		parent.InsertPairs("put_log_events_disable_retry_limit", strconv.FormatBool(*o.CloudWatch.PutLogEventsDisableRetryLimit))
	}
	if o.CloudWatch.PutLogEventsRetryLimit != nil && *o.CloudWatch.PutLogEventsRetryLimit != "" {
		parent.InsertPairs("put_log_events_retry_limit", *o.CloudWatch.PutLogEventsRetryLimit)
	}
	if o.CloudWatch.PutLogEventsRetryWait != nil && *o.CloudWatch.PutLogEventsRetryWait != "" {
		parent.InsertPairs("put_log_events_retry_wait", *o.CloudWatch.PutLogEventsRetryWait)
	}
	if o.CloudWatch.Region != nil && *o.CloudWatch.Region != "" {
		parent.InsertPairs("region", *o.CloudWatch.Region)
	}
	if o.CloudWatch.RemoveLogGroupAwsTagsKey != nil {
		parent.InsertPairs("remove_log_group_aws_tags_key", strconv.FormatBool(*o.CloudWatch.RemoveLogGroupAwsTagsKey))
	}
	if o.CloudWatch.RemoveLogGroupNameKey != nil {
		parent.InsertPairs("remove_log_group_name_key", strconv.FormatBool(*o.CloudWatch.RemoveLogGroupNameKey))
	}
	if o.CloudWatch.RemoveLogStreamNameKey != nil {
		parent.InsertPairs("remove_log_stream_name_key", strconv.FormatBool(*o.CloudWatch.RemoveLogStreamNameKey))
	}
	if o.CloudWatch.RemoveRetentionInDaysKey != nil {
		parent.InsertPairs("remove_retention_in_days_key", strconv.FormatBool(*o.CloudWatch.RemoveRetentionInDaysKey))
	}
	if o.CloudWatch.RetentionInDays != nil && *o.CloudWatch.RetentionInDays != "" {
		parent.InsertPairs("retention_in_days", *o.CloudWatch.RetentionInDays)
	}
	if o.CloudWatch.RetentionInDaysKey != nil && *o.CloudWatch.RetentionInDaysKey != "" {
		parent.InsertPairs("retention_in_days_key", *o.CloudWatch.RetentionInDaysKey)
	}
	if o.CloudWatch.UseTagAsGroup != nil && *o.CloudWatch.UseTagAsGroup != "" {
		parent.InsertPairs("use_tag_as_group", *o.CloudWatch.UseTagAsGroup)
	}
	if o.CloudWatch.UseTagAsStream != nil && *o.CloudWatch.UseTagAsStream != "" {
		parent.InsertPairs("use_tag_as_stream", *o.CloudWatch.UseTagAsStream)
	}
	if o.CloudWatch.Policy != nil && *o.CloudWatch.Policy != "" {
		parent.InsertPairs("policy", *o.CloudWatch.Policy)
	}
	if o.CloudWatch.DurationSeconds != nil && *o.CloudWatch.DurationSeconds != "" {
		parent.InsertPairs("duration_seconds", *o.CloudWatch.DurationSeconds)
	}

	// web_identity_credentials is a subsection of its own containing AWS credential settings
	child := params.NewPluginStore("web_identity_credentials")
	if o.CloudWatch.RoleARN != nil && *o.CloudWatch.RoleARN != "" {
		child.InsertPairs("role_arn", *o.CloudWatch.RoleARN)
	}
	if o.CloudWatch.WebIdentityTokenFile != nil && *o.CloudWatch.WebIdentityTokenFile != "" {
		child.InsertPairs("web_identity_token_file", *o.CloudWatch.WebIdentityTokenFile)
	}
	if o.CloudWatch.RoleSessionName != nil && *o.CloudWatch.RoleSessionName != "" {
		child.InsertPairs("role_session_name", *o.CloudWatch.RoleSessionName)
	}
	childs = append(childs, child)

	// format is a subsection of its own.  Not implemented yet.
	parent.InsertChilds(childs...)
	return parent
}

func (o *Output) stdoutPlugin(parent *params.PluginStore, loader plugins.SecretLoader) *params.PluginStore {
	return parent
}

func (o *Output) customOutput(parent *params.PluginStore, loader plugins.SecretLoader) *params.PluginStore {
	if o.CustomPlugin == nil {
		return parent
	}
	customPlugin, _ := o.CustomPlugin.Params(loader)
	parent.Content = customPlugin.Content
	return parent
}

var _ plugins.Plugin = &Output{}
