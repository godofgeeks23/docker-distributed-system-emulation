# Distributed Systems Learning Lab

This repository is a design-first scaffold for a local, single-host lab that emulates a geographically distributed system with Docker containers.

The goal is not to fake "real cloud regions." The goal is to make distributed systems behavior observable and repeatable:

- WAN latency and jitter
- packet loss, duplication, reordering, and bandwidth limits
- node crashes and pauses
- network partitions and asymmetric reachability
- replication lag and eventual consistency
- quorum behavior and consensus failure modes
- sharding, rebalancing, retries, backpressure, and convergence

The recommended architecture is documented in [docs/architecture.md](/media/godofgeeks/Baksa11/Aviral/Coding/docker-distributed-system-emulation/docs/architecture.md) and the lab catalog in [docs/lab-catalog.md](/media/godofgeeks/Baksa11/Aviral/Coding/docker-distributed-system-emulation/docs/lab-catalog.md).

## Phase 1 Scaffold

The repository now includes an initial Phase 1 implementation:

- three explicit regions: `us-east`, `eu-west`, `ap-south`
- one router container per region
- one probe container per region
- a backbone network for inter-region traffic
- Prometheus, Grafana, and cAdvisor
- a Go controller CLI for topology lifecycle, latency profile application, and a first RTT lab

## Quickstart

Requirements:

- Docker Engine with Compose v2
- permission to run containers with `NET_ADMIN`
- enough local privileges for the cAdvisor bind mounts
- Go 1.25+

Build the controller:

```bash
mkdir -p bin
go build -o bin/dslab ./cmd/dslab
```

Bring the lab up:

```bash
./bin/dslab up
```

Apply the sample geographic latency profile:

```bash
./bin/dslab apply-profile profiles/latency/us-eu-ap.yml
```

Run the baseline RTT lab:

```bash
./bin/dslab run-lab labs/00-network-basics/baseline-rtt/lab.yml
```

Reset traffic shaping:

```bash
./bin/dslab reset
```

Stop everything:

```bash
./bin/dslab down
```

Prometheus is exposed on `http://localhost:9090` and Grafana on `http://localhost:3000` with `admin/admin`.

## Feasibility

This approach is feasible and valuable for learning. It is strong for:

- network-induced distributed behavior
- failure injection
- observability and recovery analysis
- comparing consistency and replication strategies
- operating real distributed systems under controlled faults

It is weak for:

- true geographic independence
- independent hardware and kernel failure domains
- realistic disk failure semantics
- clock-skew realism
- very large clusters on modest hardware

## Recommended Direction

Do not model the lab as "a flat Docker bridge plus ad hoc `tc` on containers" once the project grows.

Instead:

1. Model regions explicitly.
2. Route inter-region traffic through router containers.
3. Apply WAN impairments on router-to-router links.
4. Describe labs declaratively with manifests, workloads, and assertions.
5. Keep observability first-class from the start.

That structure makes it much easier to add labs later without rewriting the platform.

## Proposed Repository Shape

```text
docs/
  architecture.md
  lab-catalog.md
  implementation-plan.md
```

The docs above are the current output. When implementation starts, grow the repo toward the structure described in the architecture document.
