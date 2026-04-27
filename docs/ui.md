# Web UI Plan

## Goal

Add a web UI that can:

- manage the same lifecycle controls as the terminal CLI
- show topology and lab architecture interactively
- explain the current concept with labeled nodes, links, networks, and annotations
- stay extensible as new labs, systems, and topologies are added

The UI should be a second control surface over the same platform, not a separate orchestration path.

## Design Position

The current repo already has the right foundation for this:

- a single Go control plane
- declarative lab manifests
- explicit topology metadata
- a small, understandable runtime boundary around Docker Compose and lab execution

The web UI should preserve that shape.

Recommended direction:

1. Keep `dslab` as the source of truth for orchestration.
2. Add an HTTP API and event stream in Go.
3. Build the UI as a separate frontend app during development, then bundle static assets into the Go binary for distribution.
4. Drive the visual graph from lab/topology metadata, not from hand-authored React components per lab.

## Recommended Stack

### Backend

- Go stdlib HTTP server, or `chi` if routing starts to grow
- existing `internal/runtime`, `internal/labs`, `internal/topology`, `internal/profile` packages reused by both CLI and web handlers
- `embed` for shipping the built frontend with the Go binary
- REST for commands and catalog reads
- Server-Sent Events for runtime status, timeline events, and lab progress

Why:

- the current codebase is already Go-first
- SSE is enough for one-way updates like command status, topology changes, and lab timeline events
- this avoids standing up a separate Node backend for a local single-user tool

### Frontend

- React + TypeScript
- Vite for dev/build
- `@xyflow/react` for the interactive graph canvas
- `elkjs` for auto-layout of topology and lab diagrams
- TanStack Query for server state and request lifecycle
- a small local store for UI-only canvas state; Zustand is reasonable if state starts to sprawl

Why React Flow:

- it is strong for interactive node-edge diagrams, custom nodes, zoom/pan, minimap, overlays, and selection
- it maps cleanly to regions, routers, probes, services, networks, and directional links
- it leaves room for future labs that need richer node rendering or overlays

Why ELK:

- React Flow’s own layout guidance points to ELK when graphs need compound layouts, dynamic node sizes, and routed edges
- this project needs region grouping, labeled links, and future non-trivial lab topologies

## Research Notes

Checked on April 26, 2026:

- React Flow is published under the `@xyflow/react` package and officially recommends external layout engines such as `dagre` and `elkjs`: https://reactflow.dev/learn/layouting/layouting
- The React Flow API and examples support custom node/edge rendering and interactive graph controls: https://reactflow.dev/api-reference/react-flow
- ELK Layered supports layered graph layout, compound graphs, and orthogonal routing: https://eclipse.dev/elk/reference/algorithms/org-eclipse-elk-layered.html
- Vite remains the straightforward React+TypeScript frontend setup: https://vite.dev/guide/
- Server-Sent Events remain broadly supported through `EventSource`: https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events
- TanStack Query remains the standard option for React server-state management: https://tanstack.com/query/latest/docs/react/
- `chi` remains a lightweight option if the Go HTTP surface grows beyond stdlib comfort: https://github.com/go-chi/chi

## Product Shape

Recommended first UI layout:

- left sidebar: lab catalog, profiles, topology presets
- center canvas: interactive architecture view
- right inspector: selected node/link details, controls, annotations, current fault state
- bottom drawer: timeline, logs, command output, lab artifacts, last run summary
- top bar: topology status, active profile, start/stop/reset/apply/run controls

This should feel like an operator and teaching console, not a marketing page.

## Core User Flows

### 1. Platform control

Users should be able to:

- start topology
- stop topology
- reset active network impairments
- apply a latency/fault profile
- view current service health and region status

### 2. Lab exploration

Users should be able to:

- browse labs by category and difficulty
- open a lab and inspect its topology before running it
- see which components participate in the lab
- read short concept notes and step annotations tied to the graph

### 3. Lab execution

Users should be able to:

- run a lab from the UI
- watch progress and events in real time
- inspect resulting artifacts and key metrics
- re-run the same lab with another profile or parameter set

### 4. Architecture learning

Users should be able to:

- inspect networks, routers, nodes, and services
- understand cross-region connectivity visually
- see labels for latency, packet loss, or partition state on edges
- step through the conceptual story of a lab with highlighted nodes and links

## Keep One Source of Truth

Do not let the UI invent its own topology model.

Recommended model layers:

1. `topology` data:
   defines regions, networks, nodes, services, and links
2. `profile` data:
   defines impairments applied to links or groups of links
3. `lab` data:
   defines scenario, participants, workload, assertions, and teaching metadata
4. `runtime state`:
   defines what is currently up, healthy, active, selected, running, or degraded

The existing Go packages already cover part of this. The next step is to formalize the missing metadata, not hardcode it in the frontend.

## Metadata Extension Plan

The current `lab.yml` is enough for a first CLI lab, but too small for a UI-driven catalog. Expand the manifest model so the UI can render labs without custom code.

Recommended additions:

```yaml
id: 00-network-basics/baseline-rtt
name: Baseline RTT
category: network-foundations
type: ping-matrix
summary: Measure baseline regional RTT before adding impairments.
description: |
  Learners compare raw inter-region latency before running workload labs.
tags:
  - latency
  - network
  - fundamentals
topology: three-region-probes
profiles:
  suggested:
    - profiles/latency/zero.yml
participants:
  regions: [us-east, eu-west, ap-south]
run:
  count: 4
ui:
  focus:
    nodes: [probe-us-east, probe-eu-west, probe-ap-south]
    edges: [us-east:eu-west, eu-west:ap-south, us-east:ap-south]
  annotations:
    - title: Regional RTT
      body: Each probe sends traffic to the other regions through the WAN routers.
      target: edge-group:wan
  metrics:
    primary:
      - rtt.avg_ms
      - rtt.max_ms
```

Also add a reusable topology catalog, for example:

```text
topologies/
  three-region-probes/topology.yml
  etcd-three-region/topology.yml
labs/
  00-network-basics/
    baseline-rtt/lab.yml
```

Recommended rule:

- labs reference a topology definition
- labs may add small view-specific overrides
- the UI renders from topology + lab + runtime state

That is the key extensibility move.

## Topology View Model

Introduce a backend view model that converts topology metadata into graph data the UI can consume directly.

Suggested API shape:

```json
{
  "topology_id": "three-region-probes",
  "nodes": [
    {
      "id": "region:us-east",
      "kind": "region",
      "label": "us-east",
      "parent": null,
      "position_hint": {"x": 0, "y": 0}
    },
    {
      "id": "probe-us-east",
      "kind": "probe",
      "label": "probe-us-east",
      "parent": "region:us-east",
      "status": "healthy"
    }
  ],
  "edges": [
    {
      "id": "wan:us-east:eu-west",
      "kind": "wan-link",
      "source": "router-us-east",
      "target": "router-eu-west",
      "metrics": {
        "latency_ms": 90,
        "loss_pct": 0
      }
    }
  ]
}
```

The backend should own this transformation. The frontend should not need to reverse-engineer Compose files or manifests.

## API Plan

Add a web server mode to `dslab`, for example `dslab serve`.

Recommended initial endpoints:

- `GET /api/status`
- `GET /api/topologies`
- `GET /api/topologies/{id}`
- `GET /api/labs`
- `GET /api/labs/{id}`
- `GET /api/profiles`
- `POST /api/topology/up`
- `POST /api/topology/down`
- `POST /api/topology/reset`
- `POST /api/profiles/apply`
- `POST /api/labs/run`
- `GET /api/runs`
- `GET /api/runs/{id}`
- `GET /api/events/stream`

Recommended command execution behavior:

- serialize mutating actions with a small job runner
- expose command status as `queued`, `running`, `succeeded`, `failed`
- persist recent runs and artifacts metadata in a simple local file index under `artifacts/`

This avoids race conditions like `up` and `down` being triggered concurrently from the UI.

## Event Model

Use SSE for one-way events from backend to frontend.

Suggested event types:

- `topology.status.changed`
- `profile.applied`
- `lab.run.started`
- `lab.run.progress`
- `lab.run.completed`
- `lab.run.failed`
- `service.health.changed`
- `artifact.created`

Each event should carry:

- timestamp
- event type
- human-readable summary
- structured payload

The bottom timeline drawer can render directly from this stream.

## Graph Rendering Plan

### Graph layers

Render the system in layers:

1. Region containers
2. Networks inside or between regions
3. Nodes and services inside regions
4. Edges between nodes and routers
5. Runtime overlays such as latency, packet loss, unhealthy state, selection, and teaching focus

### Node types

Start with a small set:

- `region`
- `network`
- `router`
- `probe`
- `service`
- `external-observability`

### Edge types

- `lan-link`
- `wan-link`
- `logical-replication-link`
- `fault-overlay-edge`

### Visual rules

- use labels on all important nodes and links
- use directional arrows on WAN and replication edges
- show impairment badges directly on edges
- highlight the active lab path and participants
- keep the graph read-first; editing can come later if needed

## Layout Strategy

Use deterministic automatic layout rather than hand-placing every new lab.

Recommended default:

- React Flow for interaction
- ELK Layered for layout
- left-to-right for data and WAN flow by default
- compound nodes for regions
- orthogonal routing for WAN edges where readable

Fallback:

- allow optional layout hints in topology metadata for curated views
- cache computed positions so the UI stays stable across refreshes

## Extensibility Rules

To keep future labs cheap to add:

1. New labs should usually require only metadata, workload logic, and optional annotations.
2. The graph component library should be generic and topology-driven.
3. The backend should expose normalized view models, not lab-specific JSON shapes.
4. Lab teaching content should live in manifests or nearby markdown, not in frontend code.
5. Runtime actions should always route through shared Go services used by both CLI and web API.

If a future lab needs a custom visualization, treat it as an optional panel attached to a generic lab shell, not a replacement for the common UI.

## Proposed Repository Shape

```text
cmd/
  dslab/
internal/
  api/
  catalog/
  events/
  runs/
  topology/
  labs/
  profile/
  runtime/
web/
  package.json
  vite.config.ts
  src/
    app/
    components/
    features/catalog/
    features/canvas/
    features/controls/
    features/runs/
    lib/api/
    lib/graph/
topologies/
  three-region-probes/topology.yml
docs/
  ui.md
```

Notes:

- `internal/catalog` should load labs, profiles, and topologies into a single discoverable registry
- `internal/api` should map internal models to stable HTTP DTOs
- `web/src/lib/graph` should hold node renderers, edge renderers, and ELK transform code

## Incremental Delivery Plan

### Phase 1: Shared metadata and API foundation

Deliver:

- topology catalog model
- richer lab metadata schema
- catalog loader in Go
- `dslab serve`
- read-only endpoints for labs, topologies, profiles, and status

Exit criteria:

- backend can describe the full current lab platform without scraping Compose from the frontend

### Phase 2: First usable UI

Deliver:

- React app shell
- lab catalog sidebar
- topology canvas with current three-region architecture
- status bar and basic lifecycle controls
- selected node/link inspector

Exit criteria:

- user can start/stop/reset/apply profile from the browser
- user can inspect the topology visually

### Phase 3: Real-time runs and artifacts

Deliver:

- SSE event stream
- timeline drawer
- run status badges
- lab execution panel
- artifact listing and summary views

Exit criteria:

- user can run `baseline-rtt` from the browser and inspect results without leaving the UI

### Phase 4: Teaching overlays

Deliver:

- annotation callouts
- step-based highlighting
- metric cards tied to the active lab
- profile overlays on edges

Exit criteria:

- the UI explains the lab, not just controls it

### Phase 5: Generalize for new lab families

Deliver:

- support for multiple topology templates
- support for service-specific node types
- optional custom result panels per lab family

Exit criteria:

- adding a new lab is mostly metadata plus runtime logic, with little or no new frontend plumbing

## Key Risks

### Risk: duplicated control logic between CLI and UI

Mitigation:

- move orchestration into shared service functions
- make CLI and HTTP handlers both call the same internal package APIs

### Risk: React Flow becomes the source of truth

Mitigation:

- keep React Flow as a renderer only
- generate nodes and edges from backend metadata

### Risk: new labs require frontend edits every time

Mitigation:

- define a stable topology and lab metadata contract now
- put teaching annotations in manifests

### Risk: layout becomes unstable as graphs grow

Mitigation:

- use ELK from the start
- support explicit layout hints and cached positions

### Risk: command concurrency breaks local state

Mitigation:

- use a serialized job runner for mutating actions
- publish state transitions over SSE

## Recommended First Implementation Choices

If implementation starts now, the highest-leverage choices are:

1. Add `topologies/` and expand lab metadata so the backend can describe the UI without frontend special cases.
2. Add `dslab serve` with a small HTTP API and SSE stream.
3. Build the frontend in `web/` with React, Vite, TypeScript, `@xyflow/react`, `elkjs`, and TanStack Query.
4. Reuse the current CLI runtime logic instead of shelling out from the browser layer.
5. Treat the first UI as a read-mostly operator/teaching console, not a drag-and-drop editor.

## Recommendation Summary

Build the web UI as a thin, extensible visualization and control layer on top of the existing Go control plane. Use declarative topology and lab metadata as the source for both CLI behavior and graph rendering. Use React Flow for interaction, ELK for layout, and SSE for live state. If this contract is established early, future labs can be added mostly by extending manifests and runtime logic rather than rewriting the frontend.
