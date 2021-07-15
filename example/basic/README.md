# Basic sample to configure and use Dynatrace OpenTelemetry exporter

The sample will write a single value to `otel.dynatrace.com.golang` metric. By default, the sample
will connect to the local OneAgent endpoint.

To send metric directly to Dynatrace server metric ingest, set environment variables `ENDPOINT`and `API_TOKEN`.

```bash
$ go build .

$ # Export to local OneAgent endpoint
$ ./basic
Exact
otel.dynatrace.com.golang gauge,min=1.000000,max=1.000000,sum=1.000000,count=1
Dynatrace returned: {"linesOk":1,"linesInvalid":0,"error":null}

$ # Export directly to Dynatrace server
$ ENDPOINT=https://<environment ID>.dev.dynatracelabs.com/api/v2/metrics/ingest API_TOKEN=<API Token> ./basic
```
