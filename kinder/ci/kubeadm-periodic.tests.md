# Kubeadm CI / E2E periodic tests

## Overview

Kubeadm has a set of CI tests, that you can monitor at:

https://testgrid.k8s.io/sig-cluster-lifecycle-kubeadm

The test jobs are defined here:

http://git.k8s.io/test-infra/config/jobs/kubernetes/sig-cluster-lifecycle

## Versions in scope

Kubeadm tests span across 5 Kubernetes versions:

| Version          | e.g.   |                                                              |
| ---------------- | ------ | ------------------------------------------------------------ |
| master           | v1.17  | The release under current development                        |
| current          | v1.16  | Current GA release                                           |
| current -1/minor | v1.15  | Former GA release, still officially supported                |
| current -2/minor | v1.14  | Former GA release, still officially supported for one more cycle |
| current -3/minor | v1.13  | Former GA release that is no longer supported but still tested for upgrade and skew |

Note that some tests do not span the full support skew, because they could be testing a feature that was added later
than the oldest supported version.

## Modifying e2e tests

The contents of `kinder/ci/workflows` and the test-infra Prow jobs are generated
by the `kinder/ci/tool/update-workflows` tool. To modify/add/remove the kinder workflows and/or
test-infra Jobs you must modify their templates in `kinder/ci/tool/update-workflows/templates`.

See [Managing e2e tests](../../docs/managing-e2e-tests.md) to understand more about the e2e test
setup.

## Type of tests

Kubeadm tests can be grouped in different families of tests, each one covering a different type of test workflow. Each test workflow
might be eventually repeated across all/a subset of the Kubernetes versions in scope.

### Regular tests

Kubeadm regular test are meant to create a cluster with `kubeadm init`, `kubeadm join` and then verify cluster
conformance.

Workflow file names: [`regular-*.yaml`](./workflows)

### Upgrade tests

Upgrade tests are meant to verify the proper functioning of the `kubeadm upgrade` workflow. Following upgrade tests are verified:

Workflow file names: [`upgrade-*.yaml`](./workflows)

#### Special upgrade tests

##### Tests without addon ConfigMaps

Kubeadm join and upgrade tests that ensure that kubeadm tolerates missing addon "kube-proxy" and "coredns" ConfigMaps.

Workflow file names: [`upgrade-latest-no-addon-config-maps.yaml`](./workflows)

### X on Y tests

X on Y tests are meant to verify the proper functioning of kubeadm version X with Kubernetes Y = X-1/minor.

Workflow file names: [`skew-[x]-on-[y]`](./workflows)

### kubelet X on Y tests

Kubelet X on Y tests are meant to verify the proper functioning of a version X kubelet against version Y (X+1 or X+2)
kubeadm and control plane. The coverage of X == Y is already covered by the `regular-*` tests.

Before kubeadm v1.29, the maximum kubelet skew from the kubeadm version was X-1. This required skipping
the KubeletVersion preflight check.

After kubeadm v1.29, the maximum kubelet skew from the kubeadm version has become X-3 and KubeletVersion
preflight check no longer has to be skipped.

Workflow file names: [`skew-kubelet-[x]-on-[y]`](./workflows)

### External etcd with secret copy tests

Kubeadm external etcd tests are meant to create a cluster with `kubeadm init`, `kubeadm join` using an external etcd cluster,
using kubeadm secret copy feature among control planes and then verify the cluster conformance. Currently, 1.14 is
the minimal supported version that is tested for external etcd.

Workflow file names: [`external-etcd-*`](./workflows)

### Discovery tests

Kubeadm discovery tests are meant for testing alternative discovery methods for kubeadm join. Kubernetes 1.16 is
the minimal supported version that is tested for join discovery variants.

Workflow file names: [`discovery-*`](./workflows)

### Patch tests

Kubeadm patch tests are meant for testing usage of patches with kubeadm init, join and kubeadm upgrade.
Kubernetes 1.19 is the minimal supported version that is tested.

Workflow file names: [`patches-*`](./workflows)
