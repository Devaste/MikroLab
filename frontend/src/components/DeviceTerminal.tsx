import { useEffect, useRef } from 'react';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { ApiClient } from '../api/websocket';
import 'xterm/css/xterm.css';

const PROMPT = '[admin@MikroLab] > ';

interface DeviceTerminalProps {
  apiClient: ApiClient;
  deviceId: string;
  deviceName: string;
  onClose?: () => void;
}

/**
 * Terminal component for a specific device.
 * Commands are sent to the WebSocket with the deviceId included.
 */
export default function DeviceTerminal({
  apiClient,
  deviceId,
  deviceName,
  onClose,
}: DeviceTerminalProps) {
  const terminalRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<InstanceType<typeof Terminal> | null>(null);
  const fitAddonRef = useRef<InstanceType<typeof FitAddon> | null>(null);
  const currentLineRef = useRef('');
  const cursorPosRef = useRef(0);

  useEffect(() => {
    const term = new Terminal({
      cursorBlink: true,
      cursorStyle: 'block',
      fontSize: 13,
      fontFamily: "'Consolas', 'Monaco', 'Courier New', monospace",
      theme: {
        background: '#0d1117',
        foreground: '#00ff00',
        cursor: '#00ff00',
        selectionBackground: '#003300',
        black: '#000000',
        red: '#ff4444',
        green: '#00ff00',
        yellow: '#ffff00',
        blue: '#4444ff',
        magenta: '#ff44ff',
        cyan: '#44ffff',
        white: '#ffffff',
        brightBlack: '#555555',
        brightRed: '#ff6666',
        brightGreen: '#66ff66',
        brightYellow: '#ffff66',
        brightBlue: '#6666ff',
        brightMagenta: '#ff66ff',
        brightCyan: '#66ffff',
        brightWhite: '#ffffff',
      },
      allowProposedApi: true,
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);

    xtermRef.current = term;
    fitAddonRef.current = fitAddon;

    if (terminalRef.current) {
      term.open(terminalRef.current);
    }

    // Fit after open
    const doFit = () => {
      try {
        fitAddon.fit();
      } catch {
        // ignore fit errors
      }
    };
    doFit();

    const resizeObserver = new ResizeObserver(() => doFit());
    if (terminalRef.current) {
      resizeObserver.observe(terminalRef.current);
    }

    // Write header
    term.writeln(`\r\x1b[32m=== ${deviceName} (${deviceId}) ===\x1b[0m`);
    term.writeln(`\r\x1b[90mCommands are sent to device: ${deviceId}\x1b[0m`);
    term.writeln('');
    writePrompt(term);

    // --- Terminal keyboard input ---
    term.onData((data) => {
      const code = data.charCodeAt(0);

      // Handle special keys
      if (data === '\r') {
        // Enter
        const line = currentLineRef.current.trim();
        term.writeln('');
        if (line.length > 0) {
          executeCommand(apiClient, term, line, deviceId);
        }
        currentLineRef.current = '';
        cursorPosRef.current = 0;
        writePrompt(term);
        return;
      }

      if (data === '\x7f' || data === '\b') {
        // Backspace
        if (cursorPosRef.current > 0) {
          const line = currentLineRef.current;
          currentLineRef.current =
            line.slice(0, cursorPosRef.current - 1) + line.slice(cursorPosRef.current);
          cursorPosRef.current--;
          rewriteLine(term, currentLineRef.current, cursorPosRef.current);
        }
        return;
      }

      if (code === 27) {
        // Escape sequences (arrows, etc.) - ignore for now
        return;
      }

      // Regular printable characters
      if (data.length === 1 && data >= ' ') {
        const line = currentLineRef.current;
        currentLineRef.current =
          line.slice(0, cursorPosRef.current) + data + line.slice(cursorPosRef.current);
        cursorPosRef.current++;
        rewriteLine(term, currentLineRef.current, cursorPosRef.current);
      }
    });

    return () => {
      resizeObserver.disconnect();
      term.dispose();
    };
  }, [apiClient, deviceId, deviceName]);

  return (
    <div
      style={{
        width: '100%',
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        background: '#0d1117',
      }}
    >
      {/* Title bar */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '4px 8px',
          background: '#161b22',
          borderBottom: '1px solid #00ff00',
          color: '#00ff00',
          fontFamily: "'Consolas', monospace",
          fontSize: '12px',
        }}
      >
        <span>🔌 {deviceName} ({deviceId})</span>
        {onClose && (
          <button
            onClick={onClose}
            style={{
              background: 'transparent',
              border: '1px solid #ff4444',
              color: '#ff4444',
              borderRadius: '3px',
              padding: '2px 8px',
              cursor: 'pointer',
              fontFamily: "'Consolas', monospace",
              fontSize: '11px',
            }}
          >
            ✕ Close
          </button>
        )}
      </div>

      {/* Terminal */}
      <div ref={terminalRef} style={{ flex: 1 }} />
    </div>
  );
}

/**
 * Write the prompt to the terminal at the current cursor position.
 */
function writePrompt(term: InstanceType<typeof Terminal>) {
  term.write(`\r${PROMPT}`);
}

/**
 * Rewrite the current line after editing, preserving cursor position.
 */
function rewriteLine(
  term: InstanceType<typeof Terminal>,
  currentLine: string,
  cursorPos: number,
) {
  const col = cursorPos;
  term.write(`\r\x1b[K${PROMPT}${currentLine}`);
  // Move cursor back if needed
  if (col < currentLine.length) {
    const offset = PROMPT.length + col;
    term.write(`\r\x1b[${offset}C`);
  }
}

/**
 * Send a command to the API client for the specified device and display the result.
 */
async function executeCommand(
  client: ApiClient,
  term: InstanceType<typeof Terminal>,
  line: string,
  deviceId: string,
) {
  try {
    const resp = await client.sendCommand(line, undefined, 10000, deviceId);
    if (resp.result !== undefined && resp.result !== null) {
      const output =
        typeof resp.result === 'string'
          ? resp.result
          : JSON.stringify(resp.result, null, 2);
      term.writeln(`\r${output}`);
    } else if (resp.error) {
      term.writeln(`\r\x1b[31mError: ${resp.error}\x1b[0m`);
    } else {
      term.writeln(`\r\x1b[33mEmpty response\x1b[0m`);
    }
  } catch (err) {
    term.writeln(
      `\r\x1b[31mCommand failed: ${(err as Error).message}\x1b[0m`,
    );
  }
}