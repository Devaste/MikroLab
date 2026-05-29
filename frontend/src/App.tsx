import { useCallback, useEffect, useRef, useState } from 'react';
import { ApiClient, type ConnectionStatus, type TopologyDevice } from './api/websocket';
import TopologyCanvas from './components/TopologyCanvas';
import DeviceTerminal from './components/DeviceTerminal';
import './index.css';

/**
 * Main application component with split layout:
 * - Left: Topology canvas (React Flow)
 * - Right: Terminal for the selected device
 */
export default function App() {
  const clientRef = useRef<ApiClient | null>(null);
  const [status, setStatus] = useState<ConnectionStatus>('connecting');
  const [selectedDeviceId, setSelectedDeviceId] = useState<string | null>(null);
  const [selectedDeviceName, setSelectedDeviceName] = useState<string>('Router1');
  const [devices, setDevices] = useState<TopologyDevice[]>([]);
  const [showSplash, setShowSplash] = useState(true);

  // Initialize API client once
  useEffect(() => {
    const client = new ApiClient();
    clientRef.current = client;

    const unsubStatus = client.onStatusChange((s) => {
      setStatus(s);
      if (s === 'connected') {
        setShowSplash(false);
        // Load initial devices
        refreshDevices(client);
      }
    });

    client.connect().catch((err) => {
      console.error('Failed to connect:', err);
    });

    return () => {
      unsubStatus();
      client.disconnect();
    };
  }, []);

  // Refresh device list
  const refreshDevices = useCallback(async (client?: ApiClient) => {
    const api = client || clientRef.current;
    if (!api) return;
    try {
      const resp = await api.listDevices();
      const deviceList = (resp.result as TopologyDevice[]) || [];
      setDevices(deviceList);

      // Auto-select first device if none selected
      if (!selectedDeviceId && deviceList.length > 0) {
        setSelectedDeviceId(deviceList[0].id);
        setSelectedDeviceName(deviceList[0].name);
      }
    } catch (err) {
      console.error('Failed to list devices:', err);
    }
  }, [selectedDeviceId]);

  // Handle device selection from canvas
  const handleSelectDevice = useCallback((deviceId: string) => {
    setSelectedDeviceId(deviceId);
    // Find the device name
    const dev = devices.find((d) => d.id === deviceId);
    if (dev) {
      setSelectedDeviceName(dev.name);
    }
  }, [devices]);

  // Handle opening a terminal for a device
  const handleOpenTerminal = useCallback((deviceId: string) => {
    setSelectedDeviceId(deviceId);
    const dev = devices.find((d) => d.id === deviceId);
    if (dev) {
      setSelectedDeviceName(dev.name);
    }
  }, [devices]);

  // Get current API client
  const apiClient = clientRef.current;

  // Loading/connecting splash screen
  if (showSplash) {
    return (
      <div className="splash-screen">
        <div className="splash-content">
          <h1>MikroLab</h1>
          <p>RouterOS Simulator</p>
          <p className="splash-status">
            {status === 'connecting' ? 'Connecting...' : 'Connecting to server...'}
          </p>
        </div>
      </div>
    );
  }

  if (status === 'error' && !apiClient) {
    return (
      <div className="splash-screen">
        <div className="splash-content">
          <h1 style={{ color: '#ff4444' }}>Connection Failed</h1>
          <p>Could not connect to the MikroLab server.</p>
          <p>Make sure the backend is running on port 8080.</p>
          <button
            onClick={() => window.location.reload()}
            style={{
              marginTop: '20px',
              padding: '10px 20px',
              background: '#00ff00',
              color: '#000',
              border: 'none',
              borderRadius: '4px',
              cursor: 'pointer',
              fontFamily: "'Consolas', monospace",
            }}
          >
            Retry
          </button>
        </div>
      </div>
    );
  }

  if (!apiClient) {
    return null;
  }

  return (
    <div className="app-container">
      {/* Left Panel: Topology Canvas */}
      <div className="canvas-panel">
        <TopologyCanvas
          apiClient={apiClient}
          selectedDeviceId={selectedDeviceId}
          onSelectDevice={handleSelectDevice}
          onOpenTerminal={handleOpenTerminal}
        />
      </div>

      {/* Divider */}
      <div className="divider" />

      {/* Right Panel: Device Terminal */}
      <div className="terminal-panel">
        {selectedDeviceId ? (
          <DeviceTerminal
            apiClient={apiClient}
            deviceId={selectedDeviceId}
            deviceName={selectedDeviceName}
          />
        ) : (
          <div className="no-device-selected">
            <p>Select a device from the topology canvas</p>
            <p className="hint">Double-click a device to open its terminal</p>
          </div>
        )}
      </div>
    </div>
  );
}