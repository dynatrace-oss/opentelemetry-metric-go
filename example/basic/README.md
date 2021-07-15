# Basic sample to configure and use Dynatrace OpenTelemetry exporter

The sample will write a single value to `otel.dynatrace.com.golang` metric. By default, the sample
will connect to the local OneAgent endpoint.

To send metric directly to Dynatrace server metric ingest, set environment variables `ENDPOINT`and `API_TOKEN`.

```bash
$ # Export to local OneAgent endpoint
$ ./go run main.go
2021-07-15T12:22:41.487-0400    INFO    basic/main.go:58        Using local OneAgent API
2021/07/15 12:22:41 Could not read OneAgent metadata. This is normal if no OneAgent is installed, or if you are running this on Linux.
2021-07-15T12:22:41.487-0400    DEBUG   dynatrace/dynatrace.go:221      Sending lines to Dynatrace
otel.dynatrace.com.golang,dt.metrics.source=opentelemetry gauge,min=1,max=1,sum=1,count=1
2021-07-15T12:22:41.923-0400    DEBUG   dynatrace/dynatrace.go:246      Exported 1 lines to Dynatrace

$ # Export directly to Dynatrace server
$ ENDPOINT=https://<Environment ID>.live.dynatrace.com/api/v2/metrics/ingest API_TOKEN=<API Token> go run main.go
```
