package params

type PluginName string
type InputType string
type FilterType string
type OutputType string

const (
	InputPlugin     PluginName = "input"
	ForwardPlugin   PluginName = "forward"
	HttpPlugin      PluginName = "http"
	TransportPlugin PluginName = "transport"
	InjectPlugin    PluginName = "inject"
	FormatPlugin    PluginName = "format"
	TimePlugin      PluginName = "time"
	SecurityPlugin  PluginName = "security"
	AuthPlugin      PluginName = "auth"
	UserPlugin      PluginName = "user"
	ClientPlugin    PluginName = "client"
	ServerPlugin    PluginName = "server"

	FilterPlugin            PluginName = "filter"
	GrepPlugin              PluginName = "grep"
	RecordTransformerPlugin PluginName = "record_transformer"
	ParserPlugin            PluginName = "parser"
	StdoutPlugin            PluginName = "stdout"

	ReLabelPlugin       PluginName = "relabel"
	LabelPlugin         PluginName = "label"
	LabelRouterPlugin   PluginName = "label_router"
	S3Plugin            PluginName = "s3"
	KafkaPlugin         PluginName = "kafka2"
	ElasticsearchPlugin PluginName = "elasticsearch"
	OpensearchPlugin    PluginName = "opensearch"
	MatchPlugin         PluginName = "match"
	BufferPlugin        PluginName = "buffer"
	CloudWatchPlugin    PluginName = "cloudwatch_logs"

	BufferTag    string = "tag"
	LabelTag     string = "tag"
	MatchTag     string = "tag"
	FilterTag    string = "tag"
	ProtocolName string = "protocol"
	// Default interval whitespaces between parent and child
	IntervalWhitespaces string = "  "
	DefaultFmtExpr      string = "  %s"

	// Enums the supported input types
	HttpInputType    InputType = "http"
	ForwardInputType InputType = "forward"

	// Enums the supported filter types
	RecordTransformerFilterType FilterType = "record_transformer"
	GrepFilterType              FilterType = "grep"
	ParserFilterType            FilterType = "parser"
	StdoutFilterType            FilterType = "stdout"

	// Enums the supported output types
	ForwardOutputType       OutputType = "forward"
	HttpOutputType          OutputType = "http"
	StdOutputType           OutputType = "stdout"
	KafkaOutputType         OutputType = "kafka2"
	ElasticsearchOutputType OutputType = "elasticsearch"
	OpensearchOutputType    OutputType = "opensearch"
	S3OutputType            OutputType = "s3"
	LokiOutputType          OutputType = "loki"
	CloudWatchOutputType    OutputType = "cloudwatch_logs"
)

var (
	DefaultTag        = "**"
	DefaultFormatType = "json"
	// Buffer path for single process
	DefaultBufferPath = "/buffers/fluentd/log"
)
