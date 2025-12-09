import React, { useEffect, useState, useMemo } from 'react';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import './App.css';

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
  continuous_optimization: boolean;
  stats: Stats;
}

interface Node {
  allocatable_cpu: number;
  allocatable_memory: number;
  requested_cpu: number;
  requested_memory: number;
  pods: Pod[];
  node_type: string;
  event_reason: string;
  event_message: string;
  karpenter_node_pool: string;
}

const DEFAULT_DATA_URL = 'https://autopilot-create-reco-autopilot-test-8080.tfy-usea1-ctl.devtest.truefoundry.tech/clusters/tfy-usea1-devtest/node-stats';
const DEFAULT_REFRESH_INTERVAL = 120; // seconds

const COLORS = {
  optimized: '#FFD600', // yellow
  normal: '#4CAF50',   // green
  idle: '#2196F3',     // blue
};

const App: React.FC = () => {
  const [nodes, setNodes] = useState<{ [nodeId: string]: Node }>({});
  const [loading, setLoading] = useState(true);
  const [isInitialLoad, setIsInitialLoad] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [dataUrl, setDataUrl] = useState<string>(DEFAULT_DATA_URL);
  const [refreshInterval, setRefreshInterval] = useState<number>(DEFAULT_REFRESH_INTERVAL);
  const [intervalId, setIntervalId] = useState<number | null>(null);

  const fetchData = async (url: string, isManual: boolean = false) => {
    try {
      if (isInitialLoad) {
        setLoading(true);
      } else {
        setRefreshing(true);
      }
      const response = await fetch(url);
      if (!response.ok) throw new Error('Failed to fetch data.json');
      const data = await response.json();
      setNodes(data);
      setError(null);
    } catch (err: any) {
      setError(err.message);
    } finally {
      if (isInitialLoad) {
        setLoading(false);
        setIsInitialLoad(false);
      }
      setRefreshing(false);
    }
  };

  // Fetch data on mount and when dataUrl changes
  useEffect(() => {
    fetchData(dataUrl);
    // eslint-disable-next-line
  }, [dataUrl]);

  // Set up interval for refreshing data
  useEffect(() => {
    if (intervalId) clearInterval(intervalId);
    const id = setInterval(() => {
      fetchData(dataUrl);
    }, refreshInterval * 1000);
    setIntervalId(id);
    return () => clearInterval(id);
    // eslint-disable-next-line
  }, [dataUrl, refreshInterval]);

  // Prepare data for Recharts, precompute pod colors
  const chartData = useMemo(() => {
    return (Object.entries(nodes) as [string, Node][])
      .sort(([nodeIdA], [nodeIdB]) => nodeIdA.localeCompare(nodeIdB)) // Sort by node name lexicographically
      .map(([nodeId, node]) => {
        let totalRequested = 0;
        let normalCpu = 0;
        let pods = node.pods.map((pod) => {
          totalRequested += pod.requested_cpu;
          let color = pod.name === '__idle__'
            ? COLORS.idle
            : (pod.continuous_optimization ? COLORS.optimized : COLORS.normal);
          return {
            name: pod.name,
            requested_cpu: pod.requested_cpu,
            continuous_optimization: pod.continuous_optimization,
            namespace: pod.namespace,
            color,
            p50_cpu: pod.stats?.p50_cpu ?? 0,
            max_cpu: pod.stats?.max_cpu ?? 0,
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
              continuous_optimization: false,
              namespace: '',
              color: COLORS.idle,
              p50_cpu: 0,
              max_cpu: 0,
            },
          ];
        }
        // Build podStacks for recharts
        const podStacks: Record<string, number> = {};
        // Add optimized pods as separate bars
        pods.forEach((pod) => {
          if (pod.name === '__idle__') return;
          if (pod.continuous_optimization) {
            podStacks[pod.name] = pod.requested_cpu;
          } else if (pod.name !== '__idle__') {
            normalCpu += pod.requested_cpu;
          }
        });
        // Add normal (green) block
        podStacks['normal'] = normalCpu;
        // Add idle
        const idlePod = pods.find(p => p.name === '__idle__');
        if (idlePod) {
          podStacks['__idle__'] = idlePod.requested_cpu;
        }

        const cleanedNodeId = nodeId.replaceAll('ip-', '').replaceAll('.ec2.internal', '');
        const cleanedNodePool = node.karpenter_node_pool ? node.karpenter_node_pool.replaceAll('-nodepool', '') : '';
        const displayNodeId = node.karpenter_node_pool 
          ? `${cleanedNodeId}\n${cleanedNodePool}`
          : cleanedNodeId;
        
        return {
          nodeId: displayNodeId,
          originalNodeId: nodeId,
          ...podStacks,
          pods, // only minimal info
          allocatable_cpu: node.allocatable_cpu,
          node_type: node.node_type,
          event_reason: node.event_reason,
          event_message: node.event_message,
          karpenter_node_pool: node.karpenter_node_pool,
        };
      });
  }, [nodes]);

  // Get all pod names for stacking (including idle), in the correct order
  const allPodNames = useMemo(() => {
    // Get all optimized pod names
    const optimizedNames = Array.from(
      new Set(
        chartData.flatMap((node) => node.pods.filter(p => p.continuous_optimization).map(p => p.name))
      )
    );
    // Order: green (normal) at bottom, then yellow (optimized), then blue (idle) on top
    return ['normal', ...optimizedNames, '__idle__'];
  }, [chartData]);

  // Add podColorMap to map pod names to their color
  const podColorMap = useMemo(() => {
    const map: Record<string, string> = {};
    chartData.forEach(node => {
      node.pods.forEach((pod: any) => {
        if (pod.continuous_optimization) {
          map[pod.name] = COLORS.optimized; // Always set to yellow
        }
      });
    });
    map['normal'] = COLORS.normal;
    map['__idle__'] = COLORS.idle;
    return map;
  }, [chartData]);

  // CustomTooltip as a function component inside App
  const CustomTooltip = ({ active, payload }: any) => {
    if (!active || !payload || !payload.length) return null;
    const nodePods = payload[0].payload.pods;
    // Find the node info (node_type, event_reason, event_message)
    const nodeType = payload[0].payload.node_type;
    const eventReason = payload[0].payload.event_reason;
    const eventMessage = payload[0].payload.event_message;
    const karpenterNodePool = payload[0].payload.karpenter_node_pool;
    // Sort pods: optimized, then normal, then idle
    const optimizedPods = nodePods.filter((p: any) => p.continuous_optimization);
    const normalPods = nodePods.filter((p: any) => !p.continuous_optimization && p.name !== '__idle__');
    const idlePods = nodePods.filter((p: any) => p.name === '__idle__');
    return (
      <div style={{ background: '#fff', border: '1px solid #ccc', padding: 10 }}>
        <b>{payload[0].payload.originalNodeId || payload[0].payload.nodeId}</b>
        <div style={{ fontSize: 12, color: '#555', marginBottom: 4 }}>
          <span>Type: {nodeType || '-'}</span>{' | '}
          <span>Reason: {eventReason || '-'}</span>{' | '}
          <span>Message: {eventMessage || '-'}</span>
          <span>Nodepool: {karpenterNodePool || '-'}</span>
        </div>
        <ul style={{ margin: 0, padding: 0, listStyle: 'none' }}>
          {optimizedPods.map((pod: any) => (
            <li key={pod.name}>
              <span style={{ color: pod.color }}>■</span>{' '}
              {pod.name}
              <span style={{ color: '#888', fontSize: 12 }}>({pod.namespace})</span>:&nbsp;
              {pod.requested_cpu} CPU
              {pod.max_cpu != null && pod.p50_cpu != null && pod.p50_cpu !== 0 && (
                <span style={{ color: '#888', fontSize: 12 }}>
                  {' '}(
                  {pod.max_cpu} / {pod.p50_cpu} = {Number(pod.max_cpu / pod.p50_cpu).toFixed(2)}
                  )
                </span>
              )}
            </li>
          ))}
          {normalPods.length > 0 && (
            <li style={{ fontWeight: 'bold', marginTop: 4 }}>Normal Pods:</li>
          )}
          {normalPods.map((pod: any) => (
            <li key={pod.name} style={{ marginLeft: 10 }}>
              <span style={{ color: pod.color }}>■</span>{' '}
              {pod.name}
              <span style={{ color: '#888', fontSize: 12 }}>({pod.namespace})</span>:&nbsp;
              {pod.requested_cpu} CPU
              {pod.max_cpu != null && pod.p50_cpu != null && pod.p50_cpu !== 0 && (
                <span style={{ color: '#888', fontSize: 12 }}>
                  {' '}(
                  {pod.max_cpu} / {pod.p50_cpu} = {Number(pod.max_cpu / pod.p50_cpu).toFixed(2)}
                  )
                </span>
              )}
            </li>
          ))}
          {idlePods.map((pod: any) => (
            <li key={pod.name}>
              <span style={{ color: pod.color }}>■</span>{' '}
              Idle
              <span style={{ color: '#888', fontSize: 12 }}></span>: {pod.requested_cpu} CPU
            </li>
          ))}
        </ul>
      </div>
    );
  };

  // Add a custom tick renderer for X-axis labels below the axis
  const renderXAxisTick = (props: any) => {
    const { x, y, payload } = props;
    
    // Safely handle the payload value
    const rawValue = payload?.value;
    const fullText = typeof rawValue === 'string' ? rawValue : String(rawValue || '');
    
    // Split the display value to get node ID and nodepool
    const lines = fullText.split('\n');
    const nodeId = lines[0] || '';
    const nodepool = lines[1] || '';
    
    return (
      <g transform={`translate(${x},${y})`}>
        <text x={0} y={20} textAnchor="middle" fill="#333" fontSize="12px" fontWeight="bold">
          {nodeId}
        </text>
        {nodepool && (
          <text x={0} y={32} textAnchor="middle" fill="#666" fontSize="12px" fontWeight="normal">
            [{nodepool}]
          </text>
        )}
      </g>
    );
  };

  // Custom Y-axis tick formatter to display clean numbers
  const formatYAxisTick = (value: any) => {
    const num = Number(value);
    if (num === 0) return '0';
    if (num < 1) return num.toFixed(2);
    if (num < 1000) return num.toFixed(1);
    return num.toFixed(0);
  };

  return (
    <div className="App">
      <div style={{ marginBottom: 24, display: 'flex', gap: 16, alignItems: 'center', justifyContent: 'center' }}>
        <label>
          Data URL:
          <input
            type="text"
            value={dataUrl}
            onChange={e => setDataUrl(e.target.value)}
            style={{ width: 400, marginLeft: 8 }}
          />
        </label>
        <label>
          Refresh Interval (s):
          <input
            type="number"
            min={1}
            value={refreshInterval}
            onChange={e => setRefreshInterval(Number(e.target.value))}
            style={{ width: 80, marginLeft: 8 }}
          />
        </label>
        <button onClick={() => fetchData(dataUrl, true)} style={{ marginLeft: 8 }}>Refresh Now</button>
        {refreshing && !loading && (
          <span style={{ marginLeft: 12, color: '#888', fontSize: 14 }}>Refreshing...</span>
        )}
      </div>
      <h2>Node CPU Allocation (Stacked by Pods)</h2>
      <div className={`main-content${refreshing ? ' fading' : ''}`}>
        {loading && <p>Loading...</p>}
        {error && <p style={{ color: 'red' }}>{error}</p>}
        {!loading && !error && (
          <ResponsiveContainer width="100%" height={800}>
            <BarChart data={chartData} margin={{ top: 20, right: 30, left: 20, bottom: 50 }}>
              <XAxis 
                dataKey="nodeId"
                tick={renderXAxisTick}
                interval={0}
                style={{
                  fontSize: '2rem',
                  fontWeight: 'bold',
                  textTransform: 'uppercase',
                }}
              />
              <YAxis 
                label={{ value: 'Requested CPU', angle: -90, position: 'insideLeft' }}
                domain={[0, Math.max(...Object.values(nodes).map((n: any) => n.allocatable_cpu), 1)]}
                tickFormatter={formatYAxisTick}
              />
              <Tooltip content={<CustomTooltip />} />
              {allPodNames.map((podName, idx) => (
                <Bar
                  key={podName}
                  dataKey={podName}
                  stackId="a"
                  isAnimationActive={false}
                  fill={podColorMap[podName] || '#ccc'}
                />
              ))}
            </BarChart>
          </ResponsiveContainer>
        )}
      </div>
    </div>
  );
};

export default App;
