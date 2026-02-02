# Why do we keep a copy of the OpenTelemetry Operator here?

The API structs from the `github.com/open-telemetry/opentelemetry-operator` (version `v0.143.0`) repository are manually copied here to solve issues with transitive dependencies (see [github.com/open-telemetry/opentelemetry-operator#4667](https://github.com/open-telemetry/opentelemetry-operator/issues/4667) for more information).

TODO(timuthy): Remove this copy, once dependencies in API packages have been fixed.
