# Prometheus Adapter for Kubernetes Metrics APIs

![](https://i.imgur.com/waxVImv.png)
### [View all Roadmaps](https://github.com/nholuongut/all-roadmaps) &nbsp;&middot;&nbsp; [Best Practices](https://github.com/nholuongut/all-roadmaps/blob/main/public/best-practices/) &nbsp;&middot;&nbsp; [Questions](https://www.linkedin.com/in/nholuong/)

<br/>heus and collect the appropriate metrics.

Quick Links
-----------

- [Config walkthrough](docs/config-walkthrough.md) and [config reference](docs/config.md).
- [End-to-end walkthrough](docs/walkthrough.md)
- [Deployment info and files](deploy/README.md)

Installation
-------------
If you're a helm user, a helm chart is listed on prometheus-community repository as [prometheus-community/prometheus-adapter](https://github.com/nholuongut/prometheus-adapter/issues).

To install it with the release name `my-release`, run this Helm command:

For Helm2
```console
$ helm repo add prometheus-community https://github.com/nholuongut/prometheus-helmcharts
$ helm repo update
$ helm install --name my-release prometheus-community/prometheus-adapter
```
For Helm3 ( as name is mandatory )
```console
$ helm repo add prometheus-community https://github.com/nholuongut/prometheus-helmcharts
$ helm repo update
$ helm install my-release prometheus-community/prometheus-adapter
```

Official images
---
All official images for releases after v0.8.4 are available in `registry.k8s.io/prometheus-adapter/prometheus-adapter:$VERSION`. The project also maintains a [staging registry](https://console.cloud.google.com/gcr/images/k8s-staging-prometheus-adapter/GLOBAL/) where images for each commit from the master branch are published. You can use this registry if you need to test a version from a specific commit, or if you need to deploy a patch while waiting for a new release.

Images for versions v0.8.4 and prior are only available in unofficial registries:
* https://quay.io/repository/coreos/k8s-prometheus-adapter-amd64
* https://hub.docker.com/r/directxman12/k8s-prometheus-adapter/

Configuration
-------------

The adapter takes the standard Kubernetes generic API server arguments
(including those for authentication and authorization).  By default, it
will attempt to using [Kubernetes in-cluster
config](https://kubernetes.io/docs/tasks/access-application-cluster/access-cluster/#accessing-the-api-from-a-pod)
to connect to the cluster.

It takes the following additional arguments specific to configuring how the
adapter talks to Prometheus and the main Kubernetes cluster:

- `--lister-kubeconfig=<path-to-kubeconfig>`: This configures
  how the adapter talks to a Kubernetes API server in order to list
  objects when operating with label selectors.  By default, it will use
  in-cluster config.

- `--metrics-relist-interval=<duration>`: This is the interval at which to
  update the cache of available metrics from Prometheus. By default, this
  value is set to 10 minutes.

- `--metrics-max-age=<duration>`: This is the max age of the metrics to be
  loaded from Prometheus. For example, when set to `10m`, it will query
  Prometheus for metrics since 10m ago, and only those that has datapoints
  within the time period will appear in the adapter. Therefore, the metrics-max-age
  should be equal to or larger than your Prometheus' scrape interval,
  or your metrics will occaisonally disappear from the adapter.
  By default, this is set to be the same as metrics-relist-interval to avoid
  some confusing behavior (See this [PR](https://github.com/nholuongut/prometheus-adapter/pull/230)).

  Note: We recommend setting this only if you understand what is happening.
  For example, this setting could be useful in cases where the scrape duration is
  over a network call, e.g. pulling metrics from AWS CloudWatch, or Google Monitoring,
  more specifically, Google Monitoring sometimes have delays on when data will show
  up in their system after being sampled. This means that even if you scraped data
  frequently, they might not show up soon. If you configured the relist interval to
  a short period but without configuring this, you might not be able to see your
  metrics in the adapter in certain scenarios.

- `--prometheus-url=<url>`: This is the URL used to connect to Prometheus.
  It will eventually contain query parameters to configure the connection.

- `--config=<yaml-file>` (`-c`): This configures how the adapter discovers available
  Prometheus metrics and the associated Kubernetes resources, and how it presents those
  metrics in the custom metrics API.  More information about this file can be found in
  [docs/config.md](docs/config.md).

Presentation
------------

The adapter gathers the names of available metrics from Prometheus
at a regular interval (see [Configuration](#configuration) above), and then
only exposes metrics that follow specific forms.

The rules governing this discovery are specified in a [configuration file](docs/config.md).
If you were relying on the implicit rules from the previous version of the adapter,
you can use the included `config-gen` tool to generate a configuration that matches
the old implicit ruleset:

```shell
$ go run cmd/config-gen/main.go [--rate-interval=<duration>] [--label-prefix=<prefix>]
```

### My query contains multiple metrics, how do I make that work?

It's actually fairly straightforward, if a bit non-obvious.  Simply choose one
metric to act as the "discovery" and "naming" metric, and use that to configure
the "discovery" and "naming" parts of the configuration.  Then, you can write
whichever metrics you want in the `metricsQuery`!  The series query can contain
whichever metrics you want, as long as they have the right set of labels.

For example, suppose you have two metrics `foo_total` and `foo_count`,
both with the label `system_name`, which represents the `node` resource.
Then, you might write

```yaml
rules:
- seriesQuery: 'foo_total'
  resources: {overrides: {system_name: {resource: "node"}}}
  name:
    matches: 'foo_total'
    as: 'foo'
  metricsQuery: 'sum(foo_total{<<.LabelMatchers>>}) by (<<.GroupBy>>) / sum(foo_count{<<.LabelMatchers>>}) by (<<.GroupBy>>)'
```

### I get errors about SubjectAccessReviews/system:anonymous/TLS/Certificates/RequestHeader!

It's important to understand the role of TLS in the Kubernetes cluster.  There's a high-level
overview here: https://github.com/kubernetes-incubator/apiserver-builder/blob/master/docs/concepts/auth.md.

All of the above errors generally boil down to misconfigured certificates.
Specifically, you'll need to make sure your cluster's aggregation layer is
properly configured, with requestheader certificates set up properly.

Errors about SubjectAccessReviews failing for system:anonymous generally mean
that your cluster's given requestheader CA doesn't trust the proxy certificates
from the API server aggregator.

On the other hand, if you get an error from the aggregator about invalid certificates,
it's probably because the CA specified in the `caBundle` field of your APIService
object doesn't trust the serving certificates for the adapter.

If you're seeing SubjectAccessReviews failures for non-anonymous users, check your
RBAC rules -- you probably haven't given users permission to operate on resources in
the `custom.metrics.k8s.io` API group.

### My metrics appear and disappear

You probably have a Prometheus collection interval or computation interval
that's larger than your adapter's discovery interval.  If the metrics
appear in discovery but occaisionally return not-found, those intervals
are probably larger than one of the rate windows used in one of your
queries.  The adapter only considers metrics with datapoints in the window
`[now-discoveryInterval, now]` (in order to only capture metrics that are
still present), so make sure that your discovery interval is at least as
large as your collection interval.

### I get errors when query namespace prefixed metrics?

I have namespace prefixed metrics like `{ "name": "namespaces/node_memory_PageTables_bytes", "singularName": "", "namespaced": false, "kind": "MetricValueList", "verbs": [ "get" ] }`, but I get error `Error from server (InternalError): Internal error occurred: unable to list matching resources` when access with `kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/*/node_memory_PageTables_bytes` .

Actually namespace prefixed metrics are special, we should access them with `kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/*/metrics/node_memory_PageTables_bytes`.

# 🚀 I'm are always open to your feedback.  Please contact as bellow information:
### [Contact ]
* [Name: Nho Luong]
* [Skype](luongutnho_skype)
* [Github](https://github.com/nholuongut/)
* [Linkedin](https://www.linkedin.com/in/nholuong/)
* [Email Address](luongutnho@hotmail.com)
* [PayPal.me](https://www.paypal.com/paypalme/nholuongut)

![](https://i.imgur.com/waxVImv.png)
![](Donate.png)
[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/nholuong)

# License
* Nho Luong (c). All Rights Reserved.🌟

