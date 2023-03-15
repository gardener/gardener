package common

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fluent/fluent-operator/apis/fluentd/v1alpha1/plugins"
	"github.com/fluent/fluent-operator/apis/fluentd/v1alpha1/plugins/params"
)

// +kubebuilder:object:generate:=true
// BufferCommon defines common parameters for the buffer plugin
type BufferCommon struct {
	// The @id parameter specifies a unique name for the configuration.
	Id *string `json:"id,omitempty"`
	// The @type parameter specifies the type of the plugin.
	// +kubebuilder:validation:Enum:=file;memory;file_single
	// +kubebuilder:validation:Required
	Type *string `json:"type"`
	// The @log_level parameter specifies the plugin-specific logging level
	LogLevel *string `json:"logLevel,omitempty"`
}

// Buffer defines various parameters for the buffer Plugin
type Buffer struct {
	BufferCommon `json:",inline,omitempty"`
	// The file buffer plugin
	*FileBuffer `json:",inline,omitempty"`
	// The file_single buffer plugin
	*FileSingleBuffer `json:",inline,omitempty"`
	// The time section of buffer plugin
	Time *Time `json:",inline,omitempty"`
	// Output plugin will flush chunks per specified time (enabled when time is specified in chunk keys)
	TimeKey *string `json:"timekey,omitempty"`
	// Output plugin will write chunks after timekey_wait seconds later after timekey expiration
	TimeKeyWait *string `json:"timekeyWait,omitempty"`

	// The path where buffer chunks are stored. This field would make no effect in memory buffer plugin.
	// +kubebuilder:validation:Required
	Path *string `json:"path"`
	// The output plugins group events into chunks.
	// Chunk keys, specified as the argument of <buffer> section, control how to group events into chunks.
	// If tag is empty, which means blank Chunk Keys.
	// Tag also supports Nested Field, combination of Chunk Keys, placeholders, etc.
	// See https://docs.fluentd.org/configuration/buffer-section.
	Tag string `json:"tag,omitempty"`
	// Buffer parameters
	// The max size of each chunks: events will be written into chunks until the size of chunks become this size
	// Default: 8MB (memory) / 256MB (file)
	// +kubebuilder:validation:Pattern:="^\\d+(KB|MB|GB|TB)$"
	ChunkLimitSize *string `json:"chunkLimitSize,omitempty"`
	// The max number of events that each chunks can store in it.
	// +kubebuilder:validation:Pattern:="^\\d+(KB|MB|GB|TB)$"
	ChunkLimitRecords *string `json:"chunkLimitRecords,omitempty"`
	// The size limitation of this buffer plugin instance
	// Default: 512MB (memory) / 64GB (file)
	// +kubebuilder:validation:Pattern:="^\\d+(KB|MB|GB|TB)$"
	TotalLimitSize *string `json:"totalLimitSize,omitempty"`
	// The queue length limitation of this buffer plugin instance. Default: 0.95
	// +kubebuilder:validation:Pattern:="^\\d+.?\\d+$"
	QueueLimitLength *string `json:"queueLimitLength,omitempty"`
	// Limit the number of queued chunks. Default: 1
	// If a smaller flush_interval is set, e.g. 1s,
	// there are lots of small queued chunks in the buffer.
	// With file buffer, it may consume a lot of fd resources when output destination has a problem.
	// This parameter mitigates such situations.
	// +kubebuilder:validation:Minimum:=1
	QueuedChunksLimitSize *int16 `json:"queuedChunksLimitSize,omitempty"`
	// Fluentd will decompress these compressed chunks automatically before passing them to the output plugin
	// If gzip is set, Fluentd compresses data records before writing to buffer chunks.
	// Default:text.
	// +kubebuilder:validation:Enum:=text;gzip
	Compress *string `json:"compress,omitempty"`
	// Flush parameters
	// This specifies whether to flush/write all buffer chunks on shutdown or not.
	FlushAtShutdown *bool `json:"flushAtShutdown,omitempty"`
	// FlushMode defines the flush mode:
	// lazy: flushes/writes chunks once per timekey
	// interval: flushes/writes chunks per specified time via flush_interval
	// immediate: flushes/writes chunks immediately after events are appended into chunks
	// default: equals to lazy if time is specified as chunk key, interval otherwise
	// +kubebuilder:validation:Enum:=default;lazy;interval;immediate
	FlushMode *string `json:"flushMode,omitempty"`
	// FlushInterval defines the flush interval
	// +kubebuilder:validation:Pattern:="^\\d+(\\.[0-9]{0,2})?(s|m|h|d)?$"
	FlushInterval *string `json:"flushInterval,omitempty"`
	// The sleep interval (seconds) for threads to wait for the next flush try(when no chunks are waiting)
	// +kubebuilder:validation:Pattern:="^\\d+$"
	FlushThreadCount *string `json:"flushThreadCount,omitempty"`
	// The timeout (seconds) until output plugin decides if the async write operation has failed. Default is 60s
	// +kubebuilder:validation:Pattern:="^\\d+(\\.[0-9]{0,2})?(s|m|h|d)?$"
	DelayedCommitTimeout *string `json:"delayedCommitTimeout,omitempty"`
	// OverflowAtction defines the output plugin behave when its buffer queue is full.
	// +kubebuilder:validation:Enum:throw_exception,block,drop_oldest_chunk
	// Default: throw_exception
	OverflowAction *string `json:"overflowAction,omitempty"`
	// Retry parameters
	// The maximum time (seconds) to retry to flush again the failed chunks, until the plugin discards the buffer chunks
	// +kubebuilder:validation:Pattern:="^\\d+(\\.[0-9]{0,2})?(s|m|h|d)?$"
	RetryTimeout *string `json:"retryTimeout,omitempty"`
	// If true, plugin will ignore retry_timeout and retry_max_times options and retry flushing forever.
	RetryForever *bool `json:"retryForever,omitempty"`
	// The maximum number of times to retry to flush the failed chunks. Default: none
	RetryMaxTimes *int16 `json:"retryMaxTimes,omitempty"`
	// The ratio of retry_timeout to switch to use the secondary while failing.
	// +kubebuilder:validation:Pattern:="^\\d+.?\\d+$"
	RetrySecondaryThreshold *string `json:"retrySecondaryThreshold,omitempty"`
	// Output plugin will retry periodically with fixed intervals.
	// +kubebuilder:validation:Enum:exponential_backoff,periodic
	RetryType *string `json:"retryType,omitempty"`
	// Wait in seconds before the next retry to flush or constant factor of exponential backoff
	// +kubebuilder:validation:Pattern:="^\\d+(\\.[0-9]{0,2})?(s|m|h|d)?$"
	RetryWait *string `json:"retryWait,omitempty"`
	// The base number of exponential backoff for retries.
	// +kubebuilder:validation:Pattern:="^\\d+(\\.[0-9]{0,2})?$"
	RetryExponentialBackoffBase *string `json:"retryExponentialBackoffBase,omitempty"`
	// The maximum interval (seconds) for exponential backoff between retries while failing
	// +kubebuilder:validation:Pattern:="^\\d+(\\.[0-9]{0,2})?(s|m|h|d)?$"
	RetryMaxInterval *string `json:"retryMaxInterval,omitempty"`
	// If true, the output plugin will retry after randomized interval not to do burst retries
	RetryRandomize *bool `json:"retryRandomize,omitempty"`
	// Instead of storing unrecoverable chunks in the backup directory, just discard them. This option is new in Fluentd v1.2.6.
	DisableChunkBackup *bool `json:"disableChunkBackup,omitempty"`
}

// The file buffer plugin provides a persistent buffer implementation. It uses files to store buffer chunks on disk.
type FileBuffer struct {
	// Changes the suffix of the buffer file.
	PathSuffix *string `json:"pathSuffix,omitempty"`
}

// The file_single buffer plugin is similar to file_file but it does not have the metadata file.
// See https://docs.fluentd.org/buffer/file_single#limitation
type FileSingleBuffer struct {
	// Calculates the number of records, chunk size, during chunk resume.
	CalcNumRecords *string `json:"calcNumRecords,omitempty"`
	// ChunkFormat specifies the chunk format for calc_num_records.
	// +kubebuilder:validation:Enum:=msgpack;text;auto
	ChunkFormat *string `json:"chunkFormat,omitempty"`
}

// BufferSection defines the common parameters for buffer sections
type BufferSection struct {
	// buffer section
	Buffer *Buffer `json:"buffer,omitempty"`
	// format section
	Format *Format `json:"format,omitempty"`
	// inject section
	Inject *Inject `json:"inject,omitempty"`
}

func (b *Buffer) Name() string {
	return "buffer"
}

func (b *Buffer) Params(_ plugins.SecretLoader) (*params.PluginStore, error) {
	ps := params.NewPluginStore(b.Name())
	if b.Id != nil {
		ps.InsertPairs("@id", fmt.Sprint(*b.Id))
	}
	if b.Type != nil {
		ps.InsertType(fmt.Sprint(*b.Type))
	}
	if b.LogLevel != nil {
		ps.InsertPairs("@log_level", fmt.Sprint(*b.LogLevel))
	}

	if b.FileBuffer != nil && b.FileBuffer.PathSuffix != nil {
		ps.InsertPairs("path_suffix", *b.FileBuffer.PathSuffix)
	}

	if b.FileSingleBuffer != nil {
		if b.FileSingleBuffer.CalcNumRecords != nil {
			ps.InsertPairs("calc_num_records", *b.FileSingleBuffer.CalcNumRecords)
		}
		if b.FileSingleBuffer.ChunkFormat != nil {
			ps.InsertPairs("chunk_format", *b.FileSingleBuffer.ChunkFormat)
		}
	}

	if b.Path != nil {
		if strings.HasPrefix(*b.Path, "/buffers") {
			ps.InsertPairs("path", *b.Path)
		} else {
			targetPaths := []string{"/buffers"}
			paths := strings.Split(*b.Path, "/")
			targetPaths = append(targetPaths, paths...)
			ps.InsertPairs("path", filepath.Join(targetPaths...))
		}
	} else {
		ps.InsertPairs("path", params.DefaultBufferPath)
	}

	if b.TimeKey != nil {
		ps.InsertPairs("timekey", fmt.Sprint(*b.TimeKey))
	}

	if b.TimeKeyWait != nil {
		ps.InsertPairs("timekey_wait", fmt.Sprint(*b.TimeKeyWait))
	}

	ps.InsertPairs("tag", b.Tag)

	if b.ChunkLimitSize != nil {
		ps.InsertPairs("chunk_limit_size", *b.ChunkLimitSize)
	}

	if b.ChunkLimitRecords != nil {
		ps.InsertPairs("chunk_limit_records", *b.ChunkLimitRecords)
	}

	if b.TotalLimitSize != nil {
		ps.InsertPairs("chunk_limit_size", *b.TotalLimitSize)
	}

	if b.QueueLimitLength != nil {
		ps.InsertPairs("queue_limit_length", *b.QueueLimitLength)
	}

	if b.QueuedChunksLimitSize != nil {
		ps.InsertPairs("queued_chunks_limit_size", fmt.Sprint(*b.QueuedChunksLimitSize))
	}

	if b.Compress != nil {
		ps.InsertPairs("compress", fmt.Sprint(*b.Compress))
	}

	if b.FlushAtShutdown != nil && *b.FlushAtShutdown {
		ps.InsertPairs("flush_at_shutdown", fmt.Sprint(*b.FlushAtShutdown))
	}

	if b.FlushMode != nil {
		ps.InsertPairs("flush_mode", fmt.Sprint(*b.FlushMode))
	}

	if b.FlushInterval != nil {
		ps.InsertPairs("flush_interval", fmt.Sprint(*b.FlushInterval))
	}

	if b.FlushThreadCount != nil {
		ps.InsertPairs("flush_thread_count", fmt.Sprint(*b.FlushThreadCount))
	}

	if b.DelayedCommitTimeout != nil {
		ps.InsertPairs("delayed_commit_timeout", fmt.Sprint(*b.DelayedCommitTimeout))
	}

	if b.OverflowAction != nil {
		ps.InsertPairs("overflow_action", fmt.Sprint(*b.OverflowAction))
	}

	if b.RetryTimeout != nil {
		ps.InsertPairs("retry_timeout", fmt.Sprint(*b.RetryTimeout))
	}

	if b.RetrySecondaryThreshold != nil {
		ps.InsertPairs("retry_secondary_threshold", fmt.Sprint(*b.RetryTimeout))
	}

	if b.RetryType != nil {
		ps.InsertPairs("retry_type", fmt.Sprint(*b.RetryType))
	}

	if b.RetryWait != nil {
		ps.InsertPairs("retry_wait", fmt.Sprint(*b.RetryWait))
	}

	if b.RetryExponentialBackoffBase != nil {
		ps.InsertPairs("retry_exponential_backoff_base", fmt.Sprint(*b.RetryExponentialBackoffBase))
	}

	if b.RetryMaxInterval != nil {
		ps.InsertPairs("retry_max_interval", fmt.Sprint(*b.RetryMaxInterval))
	}

	if b.RetryRandomize != nil {
		ps.InsertPairs("retry_randomize", fmt.Sprint(*b.RetryRandomize))
	}

	if b.DisableChunkBackup != nil {
		ps.InsertPairs("disable_chunk_backup", fmt.Sprint(*b.DisableChunkBackup))
	}

	if b.Time != nil {
		if b.Time.TimeType != nil {
			ps.InsertPairs("time_type", fmt.Sprint(*b.Time.TimeType))
		}
		if b.Time.TimeFormat != nil {
			ps.InsertPairs("time_type", fmt.Sprint(*b.Time.TimeFormat))
		}
		if b.Time.Localtime != nil {
			ps.InsertPairs("localtime", fmt.Sprint(*b.Time.Localtime))
		}
		if b.Time.UTC != nil {
			ps.InsertPairs("utc", fmt.Sprint(*b.Time.UTC))
		}
		if b.Time.Timezone != nil {
			ps.InsertPairs("timezone", fmt.Sprint(*b.Time.Timezone))
		}
		if b.Time.TimeFormatFallbacks != nil {
			ps.InsertPairs("time_format_fallbacks", fmt.Sprint(*b.Time.TimeFormatFallbacks))
		}
	}

	return ps, nil
}
