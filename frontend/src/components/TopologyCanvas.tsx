import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  ReactFlow,
  Controls,
  Background,
  MiniMap,
  useNodesState,
  useEdgesState,
  addEdge,
  type Connection,
  type Edge,
  type Node,
  type NodeTypes,
  MarkerType,
} from 'reactflow';
import 'reactflow/dist/style.css';
import { ApiClient, type TopologyDevice, type TopologyLink } from '../api/websocket';

// ---------------------------------------------------------------------------
// Custom Device Node
// ---------------------------------------------------------------------------

interface DeviceNodeData {
  label: string;
  deviceId: string;
  onClick?: (deviceId: string) => void;
  onDoubleClick?: (deviceId: string) => void;
}

function DeviceNode({ data }: { data: DeviceNodeData }) {
  const handleClick = useCallback(() => {
    if (data.onClick) {
      data.onClick(data.deviceId);
    }
  }, [data]);

  const handleDoubleClick = useCallback(() => {
    if (data.onDoubleClick) {
      data.onDoubleClick(data.deviceId);
    }
  }, [data]);

  return (
    <div
      onClick={handleClick}
      onDoubleClick={handleDoubleClick}
      style={{
        background: '#1a1a2e',
        color: '#00ff00',
        border: '2px solid #00ff00',
        borderRadius: '8px',
        padding: '10px 20px',
        fontFamily: "'Consolas', 'Monaco', 'Courier New', monospace",
        fontSize: '14px',
        cursor: 'pointer',
        minWidth: '120px',
        textAlign: 'center',
        boxShadow: '0 0 10px rgba(0, 255, 0, 0.3)',
        userSelect: 'none',
      }}
    >
      <div style={{ fontSize: '10px', color: '#00aa00', marginBottom: '2px' }}>RouterOS</div>
      <div style={{ fontWeight: 'bold' }}>{data.label}</div>
      <div style={{ fontSize: '9px', color: '#888', marginTop: '2px' }}>{data.deviceId}</div>
    </div>
  );
}

const nodeTypes: NodeTypes = {
  deviceNode: DeviceNode,
};

// ---------------------------------------------------------------------------
// Props
// ---------------------------------------------------------------------------

interface TopologyCanvasProps {
  apiClient: ApiClient;
  selectedDeviceId: string | null;
  onSelectDevice: (deviceId: string) => void;
  onOpenTerminal: (deviceId: string) => void;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export default function TopologyCanvas({
  apiClient,
  selectedDeviceId,
  onSelectDevice,
  onOpenTerminal,
}: TopologyCanvasProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [devices, setDevices] = useState<TopologyDevice[]>([]);
  const [links, setLinks] = useState<TopologyLink[]>([]);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const reactFlowRef = useRef<HTMLDivElement>(null);

  // Load initial topology state
  const refreshTopology = useCallback(async () => {
    setIsRefreshing(true);
    try {
      const [devicesResp, linksResp] = await Promise.all([
        apiClient.listDevices(),
        apiClient.listLinks(),
      ]);

      const deviceList = (devicesResp.result as TopologyDevice[]) || [];
      const linkList = (linksResp.result as TopologyLink[]) || [];

      setDevices(deviceList);
      setLinks(linkList);
    } catch (err) {
      console.error('Failed to refresh topology:', err);
    } finally {
      setIsRefreshing(false);
    }
  }, [apiClient]);

  // Refresh on mount
  useEffect(() => {
    refreshTopology();
  }, [refreshTopology]);

  // Update React Flow nodes when devices change
  useEffect(() => {
    const positionGrid = [
      { x: 50, y: 50 },
      { x: 350, y: 50 },
      { x: 200, y: 250 },
      { x: 50, y: 450 },
      { x: 350, y: 450 },
    ];

    const newNodes: Node[] = devices.map((dev, index) => ({
      id: dev.id,
      type: 'deviceNode',
      position: {
        x: positionGrid[index % positionGrid.length].x,
        y: positionGrid[index % positionGrid.length].y + Math.floor(index / positionGrid.length) * 300,
      },
      data: {
        label: dev.name,
        deviceId: dev.id,
        onClick: (id: string) => onSelectDevice(id),
        onDoubleClick: (id: string) => onOpenTerminal(id),
      },
      style: {
        border: selectedDeviceId === dev.id ? '2px solid #00ffff' : undefined,
        boxShadow: selectedDeviceId === dev.id ? '0 0 15px rgba(0, 255, 255, 0.5)' : undefined,
      },
    }));

    setNodes(newNodes);
  }, [devices, selectedDeviceId, onSelectDevice, onOpenTerminal, setNodes]);

  // Update React Flow edges when links change
  useEffect(() => {
    const newEdges: Edge[] = links.map((link) => ({
      id: link.id,
      source: link.deviceA,
      target: link.deviceB,
      label: `${link.interfaceA} — ${link.interfaceB}`,
      type: 'smoothstep',
      animated: true,
      style: {
        stroke: '#00ff00',
        strokeWidth: 2,
      },
      markerEnd: {
        type: MarkerType.ArrowClosed,
        color: '#00ff00',
      },
      labelStyle: {
        fill: '#00ff00',
        fontSize: 10,
        fontFamily: "'Consolas', monospace",
      },
      labelBgStyle: {
        fill: '#1a1a2e',
        fillOpacity: 0.8,
      },
    }));

    setEdges(newEdges);
  }, [links, setEdges]);

  // Handle new connections from React Flow
  const onConnect = useCallback(
    async (connection: Connection) => {
      if (!connection.source || !connection.target) return;

      // We need interface names - use "ether1" as default when connecting from canvas
      try {
        await apiClient.connectDevices(connection.source, 'ether1', connection.target, 'ether1');
        await refreshTopology();
      } catch (err) {
        console.error('Failed to connect devices:', err);
      }
    },
    [apiClient, refreshTopology],
  );

  // Handle adding a new device
  const handleAddDevice = useCallback(async () => {
    const name = `Router${devices.length + 1}`;
    try {
      await apiClient.createDevice(name);
      await refreshTopology();
    } catch (err) {
      console.error('Failed to create device:', err);
    }
  }, [apiClient, devices.length, refreshTopology]);

  // Handle deleting a device
  const handleDeleteDevice = useCallback(async () => {
    if (!selectedDeviceId) return;
    try {
      await apiClient.deleteDevice(selectedDeviceId);
      onSelectDevice('');
      await refreshTopology();
    } catch (err) {
      console.error('Failed to delete device:', err);
    }
  }, [apiClient, selectedDeviceId, onSelectDevice, refreshTopology]);

  return (
    <div style={{ width: '100%', height: '100%', display: 'flex', flexDirection: 'column' }}>
      {/* Toolbar */}
      <div
        style={{
          display: 'flex',
          gap: '8px',
          padding: '8px',
          background: '#0d0d1a',
          borderBottom: '1px solid #00ff00',
          alignItems: 'center',
        }}
      >
        <button
          onClick={handleAddDevice}
          style={buttonStyle}
          title="Add a new RouterOS device"
        >
          + Add Device
        </button>
        <button
          onClick={handleDeleteDevice}
          style={{ ...buttonStyle, opacity: selectedDeviceId ? 1 : 0.5 }}
          disabled={!selectedDeviceId}
          title="Delete selected device"
        >
          🗑 Delete
        </button>
        <button
          onClick={refreshTopology}
          style={buttonStyle}
          title="Refresh topology"
        >
          {isRefreshing ? '⟳' : '↻'} Refresh
        </button>
        <div style={{ flex: 1 }} />
        <div style={{ color: '#00ff00', fontFamily: "'Consolas', monospace", fontSize: '12px' }}>
          {devices.length} device{devices.length !== 1 ? 's' : ''} | {links.length} link{links.length !== 1 ? 's' : ''}
        </div>
      </div>

      {/* React Flow Canvas */}
      <div ref={reactFlowRef} style={{ flex: 1, background: '#0a0a1a' }}>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onConnect={onConnect}
          nodeTypes={nodeTypes}
          fitView
          fitViewOptions={{ padding: 0.2 }}
          style={{ background: '#0a0a1a' }}
          proOptions={{ hideAttribution: true }}
          defaultEdgeOptions={{
            type: 'smoothstep',
            animated: true,
            style: { stroke: '#00ff00', strokeWidth: 2 },
          }}
        >
          <Background color="#1a1a2e" gap={20} />
          <Controls
            style={{
              background: '#1a1a2e',
              border: '1px solid #00ff00',
              borderRadius: '4px',
            }}
            className="mikrolab-controls"
          />
          <MiniMap
            style={{
              background: '#0a0a1a',
              border: '1px solid #00ff00',
            }}
            nodeColor={() => '#1a1a2e'}
            maskColor="rgba(0, 0, 0, 0.7)"
          />
        </ReactFlow>
      </div>

      {/* Status bar */}
      <div
        style={{
          padding: '4px 8px',
          background: '#0d0d1a',
          borderTop: '1px solid #00ff00',
          color: '#888',
          fontFamily: "'Consolas', monospace",
          fontSize: '10px',
        }}
      >
        {selectedDeviceId
          ? `Selected: ${selectedDeviceId}`
          : 'Click a device to select | Double-click to open terminal'}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Shared button style
// ---------------------------------------------------------------------------

const buttonStyle: React.CSSProperties = {
  background: '#1a1a2e',
  color: '#00ff00',
  border: '1px solid #00ff00',
  borderRadius: '4px',
  padding: '6px 12px',
  fontFamily: "'Consolas', monospace",
  fontSize: '12px',
  cursor: 'pointer',
  transition: 'background 0.2s',
};