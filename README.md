# kubectl waste

In Kubernetes pod scheduling and load balancing decisions (I.e.: Kubernetes Cluster Autoscaler) are made based on the resource Requests allocated for every container in a Pod.

In circumstances in which best practices for pods requests allocation are not enforced (I.e.: using Admission Controller hooks), pods could request more resources than they actually ever use causing [overcommitment of nodes](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/node/resource-qos.md#qos-classes). This `kubectl` plugin lists pods that have a high allocation of resource requests but are using only a small percentage of it.

## Test

```
make test
```

## Build

```
make build
```

## Run

```
kubectl waste
```
