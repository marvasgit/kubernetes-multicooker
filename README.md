# Kubernetes Multi Cooker
Automatically taint and evict nodes with high CPU overload based on chosen Metric PSI or Avarage Load. Derived from [kubernetes-loadwatcher](https://github.com/mittwald/kubernetes-loadwatcher).

This actually started as a small extension of [kubernetes-pressurecooker](https://github.com/rtreffer/kubernetes-pressurecooker) just to do the job.But there were popping more and more things that we needed. It became multicooker once we tried to move with it to GKE and we hit a wall with it. Because google have quite a bit of different kernels on their machines. Some of them has PSI others doesnt.

The load average describes the average length of the run queue whenever a scheduling decision is made. But it does not tell us how often processes were waiting for CPU time.
The [kernel pressure metrics (psi by facebook)](https://facebookmicrosites.github.io/psi/docs/overview.html#pressure-metric-definitions) describes how often there was not enough CPU available.

There some big clound providers that doesnt support PSI metrics out of the box. I'm looking at you Google
That is why there is a flag  `-use-avarage` to choose load avarage metrics.

## Synopsis

A kubernetes node can be overcommited on CPU: there might be more processes that want more CPU than requested. This can easily happen due to variable resource usage per pod, variance in hardware or variance in pod distributions.
By default, Kubernetes will not evict Pods from a node based on CPU usage, since CPU is considered a compressible resource. However if a node does not have enough CPU resources to handle all pods it will impose additional latencies
that can be undesirable based on the workload (e.g. web/interactive traffic).

This project contains a small Kubernetes controller that watches each node's CPU pressure; when a certain threshold is exceeded, the node will be tainted (so that no additional workloads are scheduled on an already-overloaded node) and finally the controller will start to evict Pods from the node.

Pressure is more sensitive for small overloads, e.g. with pressure information it is easy to express "there is an up to 20% chance to not get CPU instantly when needed".

## How it works

This controller can be started with two threshold flags: `-taint-threshold` and `-evict-threshold`. There are also safeguard flags `-min-pod-age` and `-eviction-backoff`.
There are also few configuration flags 
Use`-use-avarage` to choose load avarage metrics instead of PSI.
Use `-target-metric` to choose the metric that will be a good fit to use for the threshold. 
Possible values are 1,2,3 based on the metric type. For LoadAva this will be [Load1, Load5,Load15] respectively for PSI it will be [Avg10,Avg60,Avg300]
The controller will continuously monitor a node's CPU pressure.

- If the target-metric exceeds the _taint threshold_, the node will be tainted with a `multicooker/load-exceeded` taint with the `PreferNoSchedule` effect. This will instruct Kubernetes to not schedule any additional workloads on this node if at all possible.
- Once node is tainted target metric is moved to the first one so the controller will be more reactive.
- If the ALL the metrics falls back below the _taint threshold_, the taint will be removed again.
- If the the FIRST Metric (Load1 Avg10) exceeds the _eviction threshold_, the controller will pick a suitable Pod running on the node and evict it. However, the following types of Pods will _not_ be evicted:

    - Pods with the `Guaranteed` QoS class
    - Pods belonging to Stateful Sets
    - Pods belonging to Daemon Sets
    - Standalone pods not managed by any kind of controller
    - Pods running in the `kube-system` namespace or with a critical `priorityClassName`
    - Pods newer than _min-pod-age_
    
After a Pod was evicted, the next Pod will be evicted after a configurable _eviction backoff_ (controllable using the `evict-backoff` argument) if the FIRST Metric (Load1 Avg10) is still above the _eviction threshold_.

Older pods will be evicted first.
The ration to remove old pods first is tat it is usually better to move well behaving pods away from bad neighbors
than moving bad neighbors through the cluster. And as a node will always stay in a healthy state it can be assumed
that the older pods are less likely to be the cause of an overload.

## Memory guard

Optionally the controller also watches the node's available memory (`MemAvailable / MemTotal` from `/proc/meminfo`, in percent). It works on every kernel, including nodes without PSI, and acts earlier and softer than kubelet's hard `memory.available` eviction.

- `-memory-taint-threshold`: when available memory drops below this percentage, the node gets a separate `multicooker/memory-pressure` taint with the `NoSchedule` effect: memory cannot be throttled like CPU, so new pods are kept off the node strictly. Other DaemonSets need a toleration for this taint to roll out onto a node while it is set. The taint is removed once available memory is back at or above the threshold. `0` (the default) disables the memory guard.
- `-memory-evict-threshold`: while the memory taint is set and available memory is below this percentage, a Pod is evicted using the same safety rules, `-evict-backoff` and `-min-pod-age` as the CPU path. Among eligible Pods, those with a container without a memory limit are evicted first. Must be lower than or equal to the taint threshold; `0` disables memory eviction.

The memory and CPU taints are independent: CPU recovery never removes the memory taint and vice versa. Both share one eviction back-off, so together they evict at most one Pod per back-off period.

In the chart this is `memoryGuard.enabled` / `memoryGuard.taintThreshold` / `memoryGuard.evictThreshold` (default on, 10% / 5%). Metrics: `multicooker_memory_available_percent`, `multicooker_memory_pressure_exceeded`.

## Installation

There is a helm chart in the repo.
To install from repo folder:

`helm upgrade --install --namespace kube-system kubernetes-multicooker chart/`
## TODO

- Create tests
- Fix prometheus metrics to be per node- release 1.0.2