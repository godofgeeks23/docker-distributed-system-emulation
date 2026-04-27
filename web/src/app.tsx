import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  applyProfile,
  getJob,
  getLab,
  getLabs,
  getProfiles,
  getStatus,
  runLab,
  startTopology,
  stopTopology,
  resetTopology,
  type ArtifactSummary,
  type BrokerEvent,
  type Job,
  type Lab,
  type Profile,
  type TopologyNode,
} from "./lib/api";
import { GraphCanvas } from "./components/graph-canvas";

export function App() {
  const queryClient = useQueryClient();
  const [selectedLabID, setSelectedLabID] = useState<string>("");
  const [selectedProfilePath, setSelectedProfilePath] = useState<string>("");
  const [selectedNode, setSelectedNode] = useState<TopologyNode | undefined>();
  const [events, setEvents] = useState<BrokerEvent[]>([]);
  const [activeJob, setActiveJob] = useState<Job | null>(null);

  const statusQuery = useQuery({
    queryKey: ["status"],
    queryFn: getStatus,
    refetchInterval: 5000,
  });
  const labsQuery = useQuery({
    queryKey: ["labs"],
    queryFn: getLabs,
  });
  const profilesQuery = useQuery({
    queryKey: ["profiles"],
    queryFn: getProfiles,
  });

  useEffect(() => {
    if (!selectedLabID && labsQuery.data?.length) {
      setSelectedLabID(labsQuery.data[0].id);
    }
  }, [labsQuery.data, selectedLabID]);

  const labDetailQuery = useQuery({
    queryKey: ["lab", selectedLabID],
    queryFn: () => getLab(selectedLabID),
    enabled: selectedLabID.length > 0,
  });

  useEffect(() => {
    if (!selectedProfilePath && profilesQuery.data?.length) {
      setSelectedProfilePath(profilesQuery.data[0].path);
    }
  }, [profilesQuery.data, selectedProfilePath]);

  useEffect(() => {
    const source = new EventSource("/api/events/stream");
    source.onmessage = () => undefined;
    source.addEventListener("job.queued", handleEvent);
    source.addEventListener("job.started", handleEvent);
    source.addEventListener("topology.up.completed", handleEvent);
    source.addEventListener("topology.down.completed", handleEvent);
    source.addEventListener("topology.reset.completed", handleEvent);
    source.addEventListener("profile.apply.completed", handleEvent);
    source.addEventListener("lab.run.completed", handleEvent);
    source.addEventListener("lab.run.failed", handleEvent);

    function handleEvent(event: MessageEvent<string>) {
      const payload = JSON.parse(event.data) as BrokerEvent;
      setEvents((current) => [payload, ...current].slice(0, 24));
      void queryClient.invalidateQueries({ queryKey: ["status"] });
      if (selectedLabID) {
        void queryClient.invalidateQueries({ queryKey: ["lab", selectedLabID] });
      }
    }

    return () => {
      source.close();
    };
  }, [queryClient, selectedLabID]);

  useEffect(() => {
    if (!activeJob || (activeJob.status !== "queued" && activeJob.status !== "running")) {
      return;
    }

    const timer = window.setInterval(() => {
      void getJob(activeJob.id).then((job) => {
        setActiveJob(job);
        if (job.status === "succeeded" || job.status === "failed") {
          void queryClient.invalidateQueries({ queryKey: ["status"] });
        }
      });
    }, 1000);

    return () => {
      window.clearInterval(timer);
    };
  }, [activeJob, queryClient]);

  const actionMutation = useMutation({
    mutationFn: async (kind: "up" | "down" | "reset" | "apply-profile" | "run-lab") => {
      if (kind === "up") return startTopology();
      if (kind === "down") return stopTopology();
      if (kind === "reset") return resetTopology();
      if (kind === "apply-profile") return applyProfile(selectedProfilePath);
      const lab = labDetailQuery.data?.lab;
      if (!lab) {
        throw new Error("No lab selected");
      }
      return runLab(lab.path);
    },
    onSuccess(job) {
      setActiveJob(job);
    },
  });

  const selectedLab = labDetailQuery.data?.lab;
  const topology = labDetailQuery.data?.topology;
  const recentRun = activeJob?.output?.artifact ?? statusQuery.data?.recent_runs[0];
  const groupedLabs = useMemo(() => groupLabs(labsQuery.data ?? []), [labsQuery.data]);

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="panel">
          <div className="panel__eyebrow">Topology</div>
          <h1 className="panel__title">dslab UI</h1>
          <p className="panel__body">
            Control the current lab runtime and inspect the active architecture from the browser.
          </p>
          <div className="status-row">
            <span className={`status-pill ${statusQuery.data?.topology_up ? "is-live" : ""}`}>
              {statusQuery.data?.topology_up ? "running" : "stopped"}
            </span>
            {statusQuery.data?.active_profile ? <span className="status-pill">{statusQuery.data.active_profile}</span> : null}
          </div>
        </div>

        <div className="panel">
          <div className="panel__eyebrow">Controls</div>
          <div className="controls-grid">
            <button onClick={() => actionMutation.mutate("up")}>Start</button>
            <button onClick={() => actionMutation.mutate("down")}>Stop</button>
            <button onClick={() => actionMutation.mutate("reset")}>Reset</button>
          </div>
          <label className="field">
            <span>Latency profile</span>
            <select value={selectedProfilePath} onChange={(event) => setSelectedProfilePath(event.target.value)}>
              {(profilesQuery.data ?? []).map((profile: Profile) => (
                <option key={profile.id} value={profile.path}>
                  {profile.name} ({profile.path})
                </option>
              ))}
            </select>
          </label>
          <button className="button--wide" onClick={() => actionMutation.mutate("apply-profile")}>
            Apply Profile
          </button>
          <button className="button--wide accent" onClick={() => actionMutation.mutate("run-lab")} disabled={!selectedLab}>
            Run Lab
          </button>
          {activeJob ? (
            <div className={`job-card job-card--${activeJob.status}`}>
              <div>{activeJob.summary}</div>
              <strong>{activeJob.status}</strong>
              {activeJob.error ? <div className="job-card__error">{activeJob.error}</div> : null}
            </div>
          ) : null}
        </div>

        <div className="panel panel--scroll">
          <div className="panel__eyebrow">Labs</div>
          {Object.entries(groupedLabs).map(([category, labs]) => (
            <div key={category} className="lab-group">
              <div className="lab-group__title">{category}</div>
              {labs.map((lab) => (
                <button
                  key={lab.id}
                  className={`lab-item ${selectedLabID === lab.id ? "is-selected" : ""}`}
                  onClick={() => setSelectedLabID(lab.id)}
                >
                  <span>{lab.name}</span>
                  {lab.summary ? <small>{lab.summary}</small> : null}
                </button>
              ))}
            </div>
          ))}
        </div>
      </aside>

      <main className="canvas-column">
        <div className="canvas-header">
          <div>
            <div className="panel__eyebrow">Architecture View</div>
            <h2>{selectedLab?.name ?? "Select a lab"}</h2>
            <p>{selectedLab?.description ?? "Choose a lab to inspect its topology and controls."}</p>
          </div>
          <div className="service-summary">
            {(statusQuery.data?.services ?? []).map((service) => (
              <span key={service.name} className={`status-pill ${service.state === "running" ? "is-live" : ""}`}>
                {service.name}:{service.state}
              </span>
            ))}
          </div>
        </div>
        <GraphCanvas topology={topology} onSelect={setSelectedNode} />
      </main>

      <aside className="inspector">
        <div className="panel">
          <div className="panel__eyebrow">Inspector</div>
          {selectedNode ? (
            <>
              <h3>{selectedNode.label}</h3>
              <dl className="detail-list">
                <div><dt>Kind</dt><dd>{selectedNode.kind}</dd></div>
                {selectedNode.service ? <div><dt>Service</dt><dd>{selectedNode.service}</dd></div> : null}
                {selectedNode.region_id ? <div><dt>Region</dt><dd>{selectedNode.region_id}</dd></div> : null}
                {selectedNode.ip ? <div><dt>IP</dt><dd>{selectedNode.ip}</dd></div> : null}
                {selectedNode.subnet ? <div><dt>Subnet</dt><dd>{selectedNode.subnet}</dd></div> : null}
              </dl>
            </>
          ) : (
            <p className="panel__body">Select a node on the graph to inspect it.</p>
          )}
        </div>

        <div className="panel">
          <div className="panel__eyebrow">Concept Notes</div>
          {(selectedLab?.ui?.annotations ?? []).map((annotation) => (
            <div key={annotation.title} className="annotation">
              <strong>{annotation.title}</strong>
              <p>{annotation.body}</p>
            </div>
          ))}
          {!(selectedLab?.ui?.annotations?.length) ? <p className="panel__body">No lab annotations yet.</p> : null}
        </div>

        <div className="panel panel--scroll">
          <div className="panel__eyebrow">Timeline</div>
          {events.map((event) => (
            <div key={event.id} className="timeline-entry">
              <strong>{event.type}</strong>
              <p>{event.summary}</p>
              <small>{new Date(event.timestamp).toLocaleString()}</small>
            </div>
          ))}
          {!events.length ? <p className="panel__body">Waiting for runtime events.</p> : null}
        </div>

        <div className="panel panel--scroll">
          <div className="panel__eyebrow">Latest Result</div>
          <ArtifactView artifact={recentRun} />
        </div>
      </aside>
    </div>
  );
}

function groupLabs(labs: Lab[]) {
  return labs.reduce<Record<string, Lab[]>>((groups, lab) => {
    const key = lab.category ?? "uncategorized";
    groups[key] = groups[key] ?? [];
    groups[key].push(lab);
    return groups;
  }, {});
}

function ArtifactView({ artifact }: { artifact?: ArtifactSummary | Record<string, unknown> | null }) {
  if (!artifact) {
    return <p className="panel__body">No artifacts yet.</p>;
  }

  if ("results" in artifact) {
    return (
      <pre className="artifact-json">{JSON.stringify(artifact, null, 2)}</pre>
    );
  }

  return <pre className="artifact-json">{JSON.stringify(artifact, null, 2)}</pre>;
}
