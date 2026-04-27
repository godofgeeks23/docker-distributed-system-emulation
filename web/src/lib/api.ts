export type StatusResponse = {
  root: string;
  topology_up: boolean;
  active_profile?: string;
  services: Array<{
    name: string;
    state: string;
    health: string;
  }>;
  recent_runs: ArtifactSummary[];
};

export type ArtifactSummary = {
  path: string;
  name: string;
  type: string;
  captured_at: string;
};

export type TopologyNode = {
  id: string;
  kind: string;
  label: string;
  parent_id?: string;
  service?: string;
  region_id?: string;
  subnet?: string;
  ip?: string;
  highlighted?: boolean;
};

export type TopologyEdge = {
  id: string;
  kind: string;
  source: string;
  target: string;
  label?: string;
  highlighted?: boolean;
};

export type TopologyView = {
  id: string;
  name: string;
  summary?: string;
  description?: string;
  nodes: TopologyNode[];
  edges: TopologyEdge[];
};

export type Lab = {
  id: string;
  name: string;
  category?: string;
  type: string;
  summary?: string;
  description?: string;
  tags?: string[];
  topology?: string;
  path: string;
  profiles?: {
    suggested?: string[];
  };
  ui?: {
    annotations?: Array<{
      title: string;
      body: string;
      target: string;
    }>;
  };
};

export type Profile = {
  id: string;
  name: string;
  path: string;
};

export type Job = {
  id: string;
  type: string;
  status: "queued" | "running" | "succeeded" | "failed";
  requested_at: string;
  started_at?: string;
  finished_at?: string;
  summary: string;
  error?: string;
  output?: {
    path?: string;
    artifact?: Record<string, unknown>;
  };
};

export type LabDetail = {
  lab: Lab;
  topology?: TopologyView;
};

export type BrokerEvent = {
  id: string;
  type: string;
  timestamp: string;
  summary: string;
  payload?: Record<string, unknown>;
};

async function request<T>(url: string, init?: RequestInit): Promise<T> {
  const response = await fetch(url, {
    headers: {
      "Content-Type": "application/json",
    },
    ...init,
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(payload.error ?? response.statusText);
  }
  return response.json() as Promise<T>;
}

export function getStatus() {
  return request<StatusResponse>("/api/status");
}

export async function getTopologies() {
  const payload = await request<{ topologies: TopologyView[] }>("/api/topologies");
  return payload.topologies;
}

export async function getLabs() {
  const payload = await request<{ labs: Lab[] }>("/api/labs");
  return payload.labs;
}

export async function getLab(id: string) {
  return request<LabDetail>(`/api/labs/${id}`);
}

export async function getProfiles() {
  const payload = await request<{ profiles: Profile[] }>("/api/profiles");
  return payload.profiles;
}

export function getJob(id: string) {
  return request<Job>(`/api/jobs/${id}`);
}

export function startTopology() {
  return request<Job>("/api/topology/up", { method: "POST" });
}

export function stopTopology() {
  return request<Job>("/api/topology/down", { method: "POST" });
}

export function resetTopology() {
  return request<Job>("/api/topology/reset", { method: "POST" });
}

export function applyProfile(path: string) {
  return request<Job>("/api/profiles/apply", {
    method: "POST",
    body: JSON.stringify({ path }),
  });
}

export function runLab(path: string) {
  return request<Job>("/api/labs/run", {
    method: "POST",
    body: JSON.stringify({ path }),
  });
}
