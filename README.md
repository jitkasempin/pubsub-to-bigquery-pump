# pubsub-to-bigquery-pump

> Warning, this service will error on execution due to Container Sandbox Limitation: Unsupported syscall setsockopt. Investigating alternative approach

[Cloud Run](https://cloud.google.com/run/) based service to drain JSON-formatted messages from PubSub topic into BigQuery table.

## Why

Google Cloud has an easy approach to draining your PubSub messages into BigQuery. Using   Dataflow and an existing template you can quickly create job that will consistently and reliably stream your messages into BigQuery.

```shell
gcloud dataflow jobs run $JOB_NAME --region us-west1 \
  --gcs-location gs://dataflow-templates/latest/PubSub_to_BigQuery \
  --parameters "inputTopic=projects/${PROJECT}/topics/${TOPIC},outputTableSpec=${PROJECT}:${DATASET}.${TABLE}"
```

This approach solves many of the common issues related to back pressure, retries, and individual insert quota limits. If you are either dealing with a consistent stream of messages or looking for fastest way of draining your PubSub messages into BigQuery this is the best solution.

The one downside of that approach is that, behind the scene, Dataflow deploys VMs into your project. While the machine types and the number of these VMs are configurable, there will always be at least one VM. That means that whether there are messages to process or not, there always is cost related to running these VMs.

In situation when you are dealing with an in-frequent message flow or you don't need these messages to immediately written into BigQuery table, you can avoid that cost by using this service.

## Pre-requirements

If you don't have one already, start by creating new project and configuring [Google Cloud SDK](https://cloud.google.com/sdk/docs/). Similarly, if you have not done so already, you will have [set up Cloud Run](https://cloud.google.com/run/docs/setup).

## Usage

> To keep this document short, I provide scripts that wrap the detail commands. You should review each one of these scripts for content to understand the individual commands

### Enable APIs

To start, you will need to enable a few GCP APIs

```shell
bin/apis
```

### Configure IAM

The deployed Cloud Run service will be follow the [principle of least privilege](https://searchsecurity.techtarget.com/definition/principle-of-least-privilege-POLP) (POLP) to ensure that both Cloud Scheduler and Cloud Run services have only the necessary rights and nothing more, we are going to create `pump-service` service accounts which will be used as Cloud Run and Cloud Scheduler service account identity.

```shell
bin/account
```

Now that we have the service account created, we can assign it the necessary policies:

* `run.invoker` - required to execute Cloud Run service
* `pubsub.viewer` - required to list and read from Cloud PubSub subscription
* `bigquery.dataOwner` - required to write/read to BigQuery table
* `logging.logWriter` - required for Stackdriver logging
* `cloudtrace.agent` - required for Stackdriver tracing
* `monitoring.metricWriter` - required to write custom metrics to Stackdriver

To grant `pump-service` account all these IAM roles run:

```shell
bin/policy
```

## Deploying Service

Now that IAM is configured, we can deploy Cloud Run service

> By default we will deploy a prebuilt image (`gcr.io/cloudylabs-public/pubsub-to-bigquery-pump:0.0.2`). If you want to build this service from source, see [Building Image](#building-image) for instructions

```shell
bin/deploy
```

A couple of worth to note deployment options. First, we deployed the `pubsub-to-bigquery-pump` service to Cloud Run with `--no-allow-unauthenticated` flag which requires the invoking identity to have the `run.invoker` role. Unauthenticated requests will be rejected BEFORE they activating your service, that means no charge for unauthenticated request attempts. The service is also deployed with `--service-account` argument which will cause the `pubsub-to-bigquery-pump` service to run under the `pump-service` service account identity.

### Cloud Schedule

With our Cloud Run service deployed, we can now configure Cloud Schedule to execute that service.

```shell
bin/schedule
```

The created schedule will execute every 30 min with following content

```json
{
    "id": "sample-import",
    "source": {
        "subscription": "pump-sub",
        "max_stall": 15
    },
    "target": {
        "dataset": "pump",
        "table": "events",
        "batch_size": 1000,
        "ignore_unknowns": true
    },
    "max_duration": 60
}
```

Schedule configuration:

* `id` is the unique ID for this job. Will be used in metrics to track the counts across executions
* `source` is the PubSub configuration
  * `subscription` is the name of already existing PubSub subscription
  * `max_stall` represents the maximum amount of time (seconds) the service will wait for new messages when the queue has been drained. Should be greater than 5 seconds
* `target` is the BigQuery configuration
  * `dataset` is the name of the existing BigQuery dataset
  * `table` is the name of the existing BigQuery table
  * `batch_size` is the size of the insert batch, every n number of messages the service will insert batch into BigQuery
  * `ignore_unknowns` indicates whether the service should error when there are fields in your JSON message on PubSun that are not represented in BigQuery table
* `max_duration` is the maximum amount of time the service should execute. The service will exit after the specified number of seconds whether there are more message or not. Should not be greater than the service `--timeout`


## Metrics

Following custom metrics are being recorded in Stackdriver for each service invocation.

* `invocation` - number of times the pump service was invoked
* `messages` - number of messages processed for each job invocation
* `duration` - total duration (in seconds) of each job invocation


## Building Image

If you prefer to build your own image you can submit a job to the Cloud Build service using the included [Dockerfile](./Dockerfile) and results in versioned, non-root container image URI which will be used to deploy your service to Cloud Run.

```shell
bin/image
```

## Cleanup

To cleanup all resources created by this sample execute

```shell
bin/cleanup
```

## Disclaimer

This is my personal project and it does not represent my employer. I take no responsibility for issues caused by this code. I do my best to ensure that everything works, but if something goes wrong, my apologies is all you will get.


