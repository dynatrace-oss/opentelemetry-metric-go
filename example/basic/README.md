# Basic sample to configure and use Dynatrace OpenTelementry exporter

The sample will write a single value to `otel.dynatrace.com.golang` metric. By default, the sample
will connect to the local Dynatrace metrics endpoint.

To send metric directly to Dynatrace server metric ingest, set environment variables `ENDPOINT`and `API_TOKEN`.

```bash
$ go build .
$ ENDPOINT=https://<environment ID>.dev.dynatracelabs.com/api/v2/metrics/ingest API_TOKEN=<API Token> ./basic
Exact
otel.dynatrace.com.golang gauge,min=1.000000,max=1.000000,sum=1.000000,count=1
Dynatrace returned: {"linesOk":1,"linesInvalid":0,"error":null}
```