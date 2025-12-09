import React, { useEffect, useState, useMemo } from 'react';
import 'chart.js/auto';
import { Bar } from 'react-chartjs-2';
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  BarElement,
  Title,
  Tooltip,
  Legend,
} from 'chart.js';
import './App.css';

ChartJS.register(CategoryScale, LinearScale, BarElement, Title, Tooltip, Legend);

interface Stats {
  workload: string;
  kind: string;
  namespace: string;
  name: string;
  cpu: number;
  max_cpu: number;
  p50_cpu: number;
  p75_cpu: number;
  p90_cpu: number;
  p99_cpu: number;
  startup_max_cpu: number;
  non_startup_max_cpu: number;
  median_replicas: number;
  cpu_later_recommendation: number;
  current_resources: {
    cpu_request: number;
    cpu_limit: number;
    memory_request: number;
    memory_limit: number;
    replicas: number;
  };
}

interface Pod {
  name: string;
  namespace: string;
  requested_cpu: number;
  requested_memory: number;
  continuousOptimization: boolean;
  stats: Stats;
}

interface Node {
  allocatable_cpu: number;
  allocatable_memory: number;
  requested_cpu: number;
  requested_memory: number;
  pods: Pod[];
}

const DATA_URL = 'http://localhost:3000/data1.json';

const COLORS = {
  optimized: '#FFD600', // yellow
  normal: '#4CAF50',   // green
  idle: '#2196F3',     // blue
};

const AppChartjs: React.FC = () => {
  const [nodes, setNodes] = useState<{ [nodeId: string]: Node }>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const response = await fetch(DATA_URL);
        if (!response.ok) throw new Error('Failed to fetch data.json');
        const data = await response.json();
        setNodes(data);
        setLoading(false);
        setError(null);
      } catch (err: any) {
        setError(err.message);
        setLoading(false);
      }
    };
    fetchData();
  }, []);

  // Prepare data for Chart.js
  const { chartData, allPodNames, podColorMap, nodeIds, podNamespaceMap } = useMemo(() => {
    const nodeEntries = Object.entries(nodes);
    const nodeIds = nodeEntries.map(([nodeId]) => nodeId);
    const podSet = new Set<string>();
    const podNamespaceMap: Record<string, string> = {};
    const podColorMap: Record<string, string> = {};
    // Build per-node pod stacks
    const perNodePods: Record<string, any[]> = {};
    nodeEntries.forEach(([nodeId, node]) => {
      let totalRequested = 0;
      let pods = node.pods.map((pod) => {
        totalRequested += pod.requested_cpu;
        let color = pod.name === '__idle__'
          ? COLORS.idle
          : (pod.continuousOptimization ? COLORS.optimized : COLORS.normal);
        podSet.add(pod.name);
        podNamespaceMap[pod.name] = pod.namespace;
        podColorMap[pod.name] = color;
        return {
          name: pod.name,
          requested_cpu: pod.requested_cpu,
          continuousOptimization: pod.continuousOptimization,
          namespace: pod.namespace,
          color,
        };
      });
      // Add idle pod if needed
      const idleCpu = Math.max(0, node.allocatable_cpu - totalRequested);
      if (idleCpu > 0) {
        pods = [
          ...pods,
          {
            name: '__idle__',
            requested_cpu: idleCpu,
            continuousOptimization: false,
            namespace: '',
            color: COLORS.idle,
          },
        ];
        podSet.add('__idle__');
        podColorMap['__idle__'] = COLORS.idle;
      }
      perNodePods[nodeId] = pods;
    });
    // Sort pod names: optimized first, then normal, then idle
    const allPodNames = Array.from(podSet).sort((aName, bName) => {
      if (aName === '__idle__') return 1;
      if (bName === '__idle__') return -1;
      const aPods = Object.values(perNodePods).flat().filter((p: any) => p.name === aName);
      const bPods = Object.values(perNodePods).flat().filter((p: any) => p.name === bName);
      const aPod = aPods[0];
      const bPod = bPods[0];
      if (aPod && bPod) {
        if (aPod.continuousOptimization && !bPod.continuousOptimization) return -1;
        if (!aPod.continuousOptimization && bPod.continuousOptimization) return 1;
      }
      return 0;
    });
    // Build chart data: each dataset is a pod, each label is a node
    const chartData = {
      labels: nodeIds,
      datasets: allPodNames.map((podName) => ({
        label: podName === '__idle__' ? 'Idle' : podName,
        data: nodeIds.map((nodeId) => {
          const pod = perNodePods[nodeId].find((p: any) => p.name === podName);
          return pod ? pod.requested_cpu : 0;
        }),
        backgroundColor: podColorMap[podName] || '#ccc',
        stack: 'cpu',
      })),
    };
    return { chartData, allPodNames, podColorMap, nodeIds, podNamespaceMap };
  }, [nodes]);

  // Custom tooltip
  const customTooltip = (context: any) => {
    const { chart, tooltip } = context;
    if (!tooltip || !tooltip.dataPoints || tooltip.dataPoints.length === 0) return null;
    const nodeIdx = tooltip.dataPoints[0].dataIndex;
    const nodeId = chartData.labels[nodeIdx];
    // Build pod info for this node
    const podInfos = chartData.datasets.map((ds: any) => ({
      name: ds.label,
      value: ds.data[nodeIdx],
      color: ds.backgroundColor,
      namespace: podNamespaceMap[ds.label === 'Idle' ? '__idle__' : ds.label] || '',
    })).filter((p: any) => p.value > 0);
    return (
      <div style={{ background: '#fff', border: '1px solid #ccc', padding: 10 }}>
        <b>{nodeId}</b>
        <ul style={{ margin: 0, padding: 0, listStyle: 'none' }}>
          {podInfos.map((pod: any) => (
            <li key={pod.name}>
              <span style={{ color: pod.color }}>■</span>{' '}
              {pod.name}
              <span style={{ color: '#888', fontSize: 12 }}>({pod.namespace})</span>: {pod.value} CPU
            </li>
          ))}
        </ul>
      </div>
    );
  };

  // Chart.js options
  const options = {
    responsive: true,
    plugins: {
      legend: {
        position: 'top' as const,
      },
      title: {
        display: true,
        text: 'Node CPU Allocation (Stacked by Pods)',
      },
      tooltip: {
        enabled: true,
        mode: 'index' as const,
        intersect: false,
      },
    },
    interaction: {
      mode: 'index' as const,
      intersect: false,
    },
    scales: {
      x: {
        stacked: true,
        ticks: {
          callback: function (val: any, idx: number, values: any) {
            // Slant the label
            return this.getLabelForValue(val);
          },
          maxRotation: 45,
          minRotation: 45,
          autoSkip: false,
        },
      },
      y: {
        stacked: true,
        title: {
          display: true,
          text: 'Requested CPU',
        },
        beginAtZero: true,
        suggestedMax: Math.max(...Object.values(nodes).map(n => n.allocatable_cpu), 1),
      },
    },
  };

  return (
    <div className="App">
      <h2>Node CPU Allocation (Stacked by Pods) - Chart.js</h2>
      {loading && <p>Loading...</p>}
      {error && <p style={{ color: 'red' }}>{error}</p>}
      {!loading && !error && (
        <div style={{ width: '100%', height: 800 }}>
          <Bar data={chartData} options={options} height={800} />
        </div>
      )}
    </div>
  );
};

export default AppChartjs; 