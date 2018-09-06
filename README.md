# NRP controller
This project provides code for NRP controller - the component allowing federating kubernetes clusters on same level according to internal cluster policies.

### Developing

`go get gitlab.com/ucsd-prp-nrp-controller`

From project folder:

`dep ensure`

`./codegen.sh`

`make buildrelease`

### Installation

`kubectl apply -f https://gitlab.com/ucsd-prp/nrp-controller/raw/master/deploy.yaml`

### Using

The controller creates 2 [CRDs](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/): 

* clusters.nrp-nautilus.io - Each CRD is an external federated cluster (or some other entity needing to federate)

* clusternamespaces.nrp-nautilus.io - the namespaces in current cluster belonging to external federated cluster

```
$ kubectl get crds | grep nrp-nautilus
clusternamespaces.nrp-nautilus.io             2018-09-06T01:42:40Z
clusters.nrp-nautilus.io                      2018-09-06T00:50:33Z
```

The clusters.nrp-nautilus.io are cluster-wide.

