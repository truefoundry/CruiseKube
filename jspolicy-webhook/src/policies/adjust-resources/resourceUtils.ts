import {V1AdmissionRequest} from "@jspolicy/types";
import {V1Pod} from "@kubernetes/client-node";

const EXCLUDED_NAMESPACES: string[] = [];
const EXCLUDED_POD_PREFIXES: string[] = [];

const STATS_URL = "https://autopilot-create-reco-autopilot-test-8080.tfy-usea1-ctl.devtest.truefoundry.tech/clusters/tfy-usea1-devtest/stats";

type WorkloadInfo = {
  kind: string,
  namespace: string | undefined,
  name: string
}

function fetchStats() {
  try {
    print("Fetching stats from " + STATS_URL);
    const response = fetchSync(STATS_URL, {
      method: "GET", headers: {
        "Content-Type": "application/json"
      }
    });

    if (!response.ok) {
      print("Failed to fetch stats, status: " + response.status + " " + response.statusText);
      return null;
    }

    return response.json();
  } catch (error) {
    if (error instanceof Error) {
      print("Error fetching stats: " + error.toString());
    } else {
      print("Error fetching stats: " + String(error));
    }
    return null;
  }
}

export function getWorkloadInfoFromPod(pod: V1Pod): WorkloadInfo | null {
  if (!pod.metadata?.ownerReferences || pod.metadata.ownerReferences.length === 0) {
    return null;
  }

  let workloadRef = null;
  for (const ownerRef of pod.metadata.ownerReferences) {
    if (ownerRef.kind === "Deployment" || ownerRef.kind === "StatefulSet" || ownerRef.kind === "DaemonSet") {
      workloadRef = ownerRef;
      break;
    }
    if (ownerRef.kind === "ReplicaSet") {
      workloadRef = {
        kind: "Deployment", name: ownerRef.name.replace(/-[a-z0-9]+$/, ""),
      };
    }
  }

  if (!workloadRef) {
    return null;
  }

  return {
    kind: workloadRef.kind, namespace: pod.metadata.namespace, name: workloadRef.name
  };
}

export function findWorkloadStat(workloadInfo: WorkloadInfo) {
  const stats = fetchStats();
  // @ts-ignore
  if (!stats || !stats.stats) {
    print("Could not fetch stats, allowing pod without adjustment");
    exit();
  }

  const identifier = workloadInfo.kind + "/" + workloadInfo.namespace + "/" + workloadInfo.name;

  // @ts-ignore
  for (const stat of stats.stats) {
    if (stat.workload === identifier) {
      return stat;
    }
  }

  return null;
}

export function parseCPUToMillicores(cpuString: string): number {
  if (!cpuString) return 0;
  const cleaned = cpuString.trim();

  if (cleaned.endsWith('m')) {
    return parseInt(cleaned.slice(0, -1));
  }

  const cores = parseFloat(cleaned);
  if (isNaN(cores)) {
    print("Warning: Invalid CPU value '" + cpuString + "', treating as 0");
    return 0;
  }

  return Math.round(cores * 1000);
}

export function cpuCoresToMillicores(cpuCores: number): string {
    return Math.round(cpuCores * 1000) + "m";
}

export function parseMemoryToBytes(memoryString: string): number {
  if (!memoryString) return 0;

  const cleaned = memoryString.trim();

  if (/^\d+$/.test(cleaned)) {
    return parseInt(cleaned);
  }

  const match = cleaned.match(/^(\d*\.?\d+)([A-Za-z]*)$/);
  if (!match) return 0;

  const value = parseFloat(match[1]);
  const unit = match[2];

  switch (unit) {
    case 'K':
      return Math.round(value * 1000);
    case 'Ki':
      return Math.round(value * 1024);
    case 'M':
      return Math.round(value * 1000 * 1000);
    case 'Mi':
      return Math.round(value * 1024 * 1024);
    case 'G':
      return Math.round(value * 1000 * 1000 * 1000);
    case 'Gi':
      return Math.round(value * 1024 * 1024 * 1024);
    case 'T':
      return Math.round(value * 1000 * 1000 * 1000 * 1000);
    case 'Ti':
      return Math.round(value * 1024 * 1024 * 1024 * 1024);
    case '':
      return Math.round(value);
    default:
      print("Warning: Unknown memory unit '" + unit + "', treating as bytes");
      return Math.round(value);
  }
}

export function memoryBytesToMB(memoryBytes: number) {
    return Math.round(memoryBytes / (1000 * 1000)) + "M";
}

export function adjustResources(request: V1AdmissionRequest): [boolean, V1Pod] {
  if (!request.object) {
    print("No pod object found in request, allowing without adjustment");
    return [false, {}];
  }

  const podObject = request.object as V1Pod;
  const podName = (podObject.metadata?.name || podObject.metadata?.generateName) as string;
  const podNamespace = request.namespace as string;

  print("Processing pod: " + podNamespace + "/" + podName);

  if (EXCLUDED_NAMESPACES.includes(podNamespace)) {
    print("Skipping pod in excluded namespace: " + podNamespace);
    exit();
  }

  for (const prefix of EXCLUDED_POD_PREFIXES) {
    if (podName.startsWith(prefix)) {
      print("Skipping pod with excluded prefix: " + podName);
      exit();
    }
  }

  const workloadInfo = getWorkloadInfoFromPod(podObject);
  if (!workloadInfo) {
    print("Could not determine workload for pod " + podNamespace + "/" + podName + ", allowing without adjustment");
    return [false, {}];
  }

  print("Pod " + podNamespace + "/" + podName + " belongs to workload: " + workloadInfo.kind + "/" + workloadInfo.namespace + "/" + workloadInfo.name);

  const workloadStat = findWorkloadStat(workloadInfo);

  if (workloadStat && workloadStat?.is_horizontally_autoscaled_on_cpu) {
    print("Workload is horizontally autoscaled on CPU, skipping");
    return [false, {}];
  }

  let containers = podObject.spec?.containers || [];
  let initContainers = podObject.spec?.initContainers || [];
  let allContainers = [...containers, ...initContainers];

  let changed = false;

  if (podObject.spec?.topologySpreadConstraints && podObject.spec.topologySpreadConstraints.length > 0) {
    print("Removing " + podObject.spec.topologySpreadConstraints.length + " topologySpreadConstraints from pod " + podNamespace + "/" + podName);
    delete podObject.spec.topologySpreadConstraints;
    changed = true;
  }

  if (podObject.spec?.affinity && podObject.spec.affinity.podAntiAffinity) {
    print("Removing podAntiAffinity from pod " + podNamespace + "/" + podName);
    delete podObject.spec.affinity.podAntiAffinity;

    if (!podObject.spec.affinity.nodeAffinity && !podObject.spec.affinity.podAffinity) {
      delete podObject.spec.affinity;
    }

    changed = true;
  }

  for (const container of allContainers) {
    if (!container.resources) {
      container.resources = {};
    }
    if (!container.resources.requests) {
      container.resources.requests = {};
    }
    if (!container.resources.limits) {
      container.resources.limits = {};
    }

    if (!workloadStat) {
      delete container.resources.limits.cpu;
      changed = true
      continue;
    }

    let containerStat = null;
    for (const stat of workloadStat.container_stats) {
      if (stat.container_name === container.name) {
        containerStat = stat;
        break;
      }
    }

    if (!containerStat || containerStat.cpu_stats === null || containerStat.memory_stats === null) {
      print("No stat found for container: " + container.name + " in workload: " + workloadInfo.kind + "/" + workloadInfo.namespace + "/" + workloadInfo.name);
      continue;
    }

    let recommendedCPU = containerStat.cpu_stats.max;
    if (containerStat.simple_predictions_cpu && containerStat.simple_predictions_cpu.max_value) {
      recommendedCPU = containerStat.simple_predictions_cpu.max_value;
    }

    let recommendedMemory = containerStat.memory_stats.max;
    if (containerStat.simple_predictions_memory && containerStat.simple_predictions_memory.max_value) {
      recommendedMemory = containerStat.simple_predictions_memory.max_value;
    }

    let recommendedMemoryLimit = 2 * recommendedMemory;
    if (containerStat.memory_7day && containerStat.memory_7day.max) {
      recommendedMemoryLimit = Math.max(2 * containerStat.memory_7day.max, recommendedMemoryLimit);
    }
    const recommendedMemoryLimitBytes = Math.round(Math.max(recommendedMemoryLimit, 16) * 1000 * 1000);

    print("Container " + container.name + " - Recommended CPU: " + cpuCoresToMillicores(recommendedCPU) + " (max: " + containerStat.cpu_stats.max + ")");
    print("Container " + container.name + " - Recommended Memory: " + memoryBytesToMB(recommendedMemory));

    const currentCPUMillicores = parseCPUToMillicores(container.resources.requests.cpu);
    const recommendedCPUMillicores = Math.max(Math.round(recommendedCPU * 1000), 1);

    if (currentCPUMillicores > 0 && Math.abs(recommendedCPUMillicores - currentCPUMillicores) >= 0) {
      if (workloadInfo.kind === "DaemonSet") {
        const currentCPULimitMillicores = parseCPUToMillicores(container.resources.limits.cpu);
        const newCPULimitMillicores = recommendedCPUMillicores * 2;

        const finalCPULimitMillicores = Math.max(newCPULimitMillicores, currentCPULimitMillicores);

        delete container.resources.limits.cpu
        changed = true;
        print("Adjusted CPU limit only for DaemonSet container " + container.name + " to " + finalCPULimitMillicores + "m (request unchanged: " + currentCPUMillicores + "m)");
      } else {
        container.resources.requests.cpu = recommendedCPUMillicores + "m";
        delete container.resources.limits.cpu;
        changed = true;
        print("Adjusted CPU for container " + container.name + " from " + currentCPUMillicores + "m to " + recommendedCPUMillicores + "m");
      }
    }

    const currentMemoryBytes = parseMemoryToBytes(container.resources.requests.memory);
    const recommendedMemoryBytes = Math.round(recommendedMemory * 1000 * 1000);
    const thresholdBytes = 16 * 1000 * 1000;

    if (currentMemoryBytes > 0 && Math.abs(recommendedMemoryBytes - currentMemoryBytes) > thresholdBytes) {
      if (workloadInfo.kind === "DaemonSet") {
        const currentMemoryLimit = parseMemoryToBytes(container.resources.limits.memory);
        const finalMemoryLimitBytes = Math.max(currentMemoryLimit, recommendedMemoryLimitBytes, 16 * 1000 * 1000);

        container.resources.limits.memory = memoryBytesToMB(finalMemoryLimitBytes);
        changed = true;

        const currentMemoryMB = Math.round(currentMemoryBytes / (1000 * 1000));
        // const recommendedMemoryMB = Math.round(recommendedMemoryBytes / (1000 * 1000));

        print("Adjusted Memory limit only for DaemonSet container " + container.name + " to " + memoryBytesToMB(finalMemoryLimitBytes) + "MB (request unchanged: " + currentMemoryMB + "MB)");
      } else {
        container.resources.requests.memory = memoryBytesToMB(recommendedMemoryBytes);
        container.resources.limits.memory = memoryBytesToMB(recommendedMemoryLimitBytes);
        changed = true;

        const currentMemoryMB = Math.round(currentMemoryBytes / (1000 * 1000));
        const recommendedMemoryMB = Math.round(recommendedMemoryBytes / (1000 * 1000));
        print("Adjusted Memory for container " + container.name + " from " + currentMemoryMB + "MB to " + recommendedMemoryMB + "MB");
      }
    }
  }

  return [changed, podObject || {}];
}
