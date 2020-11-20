# Dynatrace OpenTelemetry Metrics Exporter for Go

> This project is developed and maintained by Dynatrace R&D.
Currently, this is a prototype and not intended for production use.
It is not covered by Dynatrace support.

This exporter plugs into the OpenTelemetry Metrics SDK for Go, which is in alpha/preview state and neither considered stable nor complete as of this writing.

See [open-telemetry/opentelemetry-go](https://github.com/open-telemetry/opentelemetry-go) for the current state of the OpenTelemetry SDK for Go.
## Getting started

The general setup of OpenTelemetry Go is explained in the official [Getting Started Guide](https://github.com/open-telemetry/opentelemetry-go/blob/master/README.md#getting-started).

A more detailed guide on metrics is expected to be added to the OpenTelemetry Go repository once the Metrics API and SDK are further developed.

The Dynatrace exporter is added and set up like this:

```go
  opts := dynatrace.Options{}
  if token, exists := os.LookupEnv("API_TOKEN"); exists {
    opts.APIToken = token
    opts.URL = os.Getenv("ENDPOINT")
  }

  exporter, err := dynatrace.NewExporter(opts)
  if err != nil{
    panic(err)
  }
  defer exporter.Close()

  processor := basic.New(
    simple.NewWithExactDistribution(),
    exporter,
  )

  pusher := push.New(
    processor,
    exporter,
  )
  pusher.Start()
  defer pusher.Stop()

  global.SetMeterProvider(pusher.MeterProvider())
  meter := global.Meter("otel.dynatrace.com/basic")
  vr := metric.Must(meter).NewFloat64ValueRecorder("otel.dynatrace.com.golang")
  vr.Record(context.Background(), 1.0)
```

A full setup is provided in our [example project](./example/basic/).

### Configuration

The exporter allows for configuring the following settings by setting them on the `dynatrace.Options` struct:

#### Dynatrace API Endpoint

The endpoint to which the metrics are sent is specified using the `URL` field.

Given an environment ID `myenv123` on Dynatrace SaaS, the [metrics ingest endpoint](https://www.dynatrace.com/support/help/dynatrace-api/environment-api/metric-v2/post-ingest-metrics/) would be `https://myenv123.live.dynatrace.com/api/v2/metrics/ingest`.

If a OneAgent is installed on the host, it can provide a local endpoint for providing metrics directly without the need for an API token.
This feature is currently in an Early Adopter phase and has to be enabled as described in the [OneAgent metric API documentation](https://www.dynatrace.com/support/help/how-to-use-dynatrace/metrics/metric-ingestion/ingestion-methods/local-api/).
Using the local API endpoint, the host ID and host name context are automatically added to each metric as dimensions.
The default metric API endpoint exposed by the OneAgent is `http://localhost:14499/metrics/ingest`.

#### Dynatrace API Token

The Dynatrace API token to be used by the exporter is specified using the `APIToken` field and could, for example, be read from an environment variable.

Creating an API token for your Dynatrace environment is described in the [Dynatrace API documentation](https://www.dynatrace.com/support/help/dynatrace-api/basics/dynatrace-api-authentication/).
The scope required for sending metrics is the `Ingest metrics` scope in the **API v2** section:

![API token creation](docs/img/api_token.png)

#### Metric Key Prefix

The `Prefix` field specifies an optional prefix, which is prepended to each metric key, separated by a dot (`<prefix>.<namespace>.<name>`).

#### Default Labels/Dimensions

The `Tags` field can be used to optionally specify a list of key/value pairs, which will be added as additional labels/dimensions to all data points.
