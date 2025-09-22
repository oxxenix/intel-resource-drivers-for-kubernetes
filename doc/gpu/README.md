# Intel GPU resource driver for Kubernetes

CAUTION: This is an beta / non-production software, do not use on production clusters.

## About resource driver

With structured parameters (K8s v1.31+), the DRA driver publishes ResourceSlice, scheduler allocates
the resoruces and DRA driver kubelet-plugin ensures that the allocated devices are prepared
and available for Pods.

DRA API graduated to GA with v1 API in K8s v1.34. Latest DRA drivers support only K8s v1.34+.

## Supported GPU devices (with Linux kernel Intel `i915`, `Xe` GPU drivers):
- Intel® Data Center GPU Max Series
- Intel® Data Center GPU Flex Series
- Intel® Arc A-Series, B-Series
- Intel® Iris® Xe MAX
- Intel® Integrated graphics

## Supported Kubernetes Versions

Supported Kubernetes versions are listed below:

| Branch            | Kubernetes branch/version        | Status      | DRA                            |
|:------------------|:---------------------------------|:------------|:-------------------------------|
| v0.1.0-beta       | Kubernetes v1.26 branch v1.26.x  | unsupported | Classic                        |
| v0.1.1-beta       | Kubernetes v1.27 branch v1.27.x  | unsupported | Classic                        |
| v0.2.0            | Kubernetes v1.28 branch v1.28.x  | unsupported | Classic                        |
| v0.3.0            | Kubernetes v1.28+                | unsupported | Classic                        |
| v0.4.0            | Kubernetes v1.28+                | unsupported | Classic                        |
| v0.5.0            | Kubernetes v1.27 - v1.30         | unsupported | Classic, Structured Parameters |
| v0.6.0            | Kubernetes v1.31                 | unsupported | Structured Parameters          |
| v0.7.0            | Kubernetes v1.32+                | unsupported | Structured Parameters          |
| v0.8.0            | Kubernetes v1.33-v1.34           | supported   | Structured Parameters          |
| v0.9.0            | Kubernetes v1.34+                | supported   | Structured Parameters          |

## Documentation

- [How to setup a Kubernetes cluster with DRA enabled](../CLUSTER_SETUP.md)
- [How to deploy and use Intel GPU resource driver](USAGE.md)
- Optional: [How to build Intel GPU resource driver container image](BUILD.md)
