# Autoscaling Quay Registry

Quay is capable of running at massive scale (see quay.io) with minimal configuration changes. The Quay Operator supports intelligent application scaling based on resource consumption and in the future, custom traffic and Prometheus metrics.

## HorizontalPodAutoscaler Managed Component

By default, the Operator will create a `HorizontalPodAutoscaler` for the Quay app `Deployment`, which is a [Kubernetes native API](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/). This will maintain the correct number of Quay `Pods` to meet the resource demands of the application.

### Disabling Autoscaling

If for some reason you wish to disable autoscaling or create your own `HorizontalPodAutoscaler`, simply specify the component as unmanaged in the `QuayRegistry` instance:

```yaml
apiVersion: quay.redhat.com/v1
kind: QuayRegistry
metadata:
  name: some-quay
spec:
  components:
    - kind: horizontalpodautoscaler
      managed: false
```
