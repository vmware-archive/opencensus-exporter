# Wavefront OpenCensus Go Exporter
<!-- LOGO HERE -->

This exporter provides OpenCensus trace and stats support to push metrics, histograms and traces into Wavefront.

It builds on the [Wavefront Go SDK](https://github.com/wavefrontHQ/wavefront-sdk-go).

### Usage

1. Import the SDK and Exporter packages.
    ```go
    import (
        "github.com/wavefronthq/wavefront-sdk-go/senders"
        "github.com/wavefronthq/opencensus-exporter/wavefront"
        "go.opencensus.io/stats/view"
	    "go.opencensus.io/trace"
    )
    ```

2. Initialize the [Sender](https://github.com/wavefrontHQ/wavefront-sdk-go#usage) and Exporter.
    ```go
    sender, _ := senders.NewProxySender(senders.ProxyConfiguration{/*...*/})
    exporter, _ = wavefront.NewExporter(sender, /*options...*/)

    defer func() {  // Flush before application exits
        exporter.Stop()
        sender.Close()
    }()
    ```
    The exporter supports functional options. See [Exporter Options](#exporter-options)

3. Register the exporter
    ```go
    trace.RegisterExporter(exporter)    // for exporting traces
    view.RegisterExporter(exporter)     // for exporting metrics
    ```

4. Instrument your code using OpenCensus. Learn more at [https://opencensus.io/quickstart/go/](https://opencensus.io/quickstart/go/) 

### Exporter Options

| Option                                  | Description                                                                                                                       |
|-----------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------|
| `Source(string)`                        | Overrides the source tag that is sent in Metrics and Traces                                                                       |
| `QueueSize(int)`                        | Sets the maximum number of metrics and spans queued before new ones are dropped. QueueSize must be >= 0                           |
| `AppTags(application.Tags)`             | Sets the application tags. See example and SDK for more info                                                                      |
| `Granularity(histogram.Granularity...)` | Sets the histogram Granularities that must be sent. See [SDK docs](https://github.com/wavefrontHQ/wavefront-sdk-go#distributions) |
| `DisableSelfHealth()`                   | Disables reporting exporter health such as dropped metrics, spans, etc...                                                         |
| `VerboseLogging()`                      | Logs individual errors to stderr                                                                                                  |


See [examples folder](https://github.com/wavefrontHQ/opencensus-exporter/examples) for a complete example.

### Requirements
- Go 1.9 or higher

### Links

- https://opencensus.io/exporters/supported-exporters/go/wavefront/