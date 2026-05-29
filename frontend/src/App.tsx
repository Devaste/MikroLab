import { useEffect, useRef, useState } from 'react';
import { Terminal } from 'xterm';
import { FitAddon } from 'xterm-addon-fit';
import { ApiClient, type ConnectionStatus } from './api/websocket';
import 'xterm/css/xterm.css';

const PROMPT = '[admin@MikroLab] > ';

/**
 * Full-page terminal component that connects via WebSocket to the MikroLab
 * simulator and displays a RouterOS-style CLI.
 */
export default function App() {
  const terminalRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<InstanceType<typeof Terminal> | null>(null);
  const fitAddonRef = useRef<InstanceType<typeof FitAddon> | null>(null);
  const clientRef = useRef<ApiClient | null>(null);
  const currentLineRef = useRef('');
  const cursorPosRef = useRef(0);

  const [status, setStatus] = useState<ConnectionStatus>('connecting');

  useEffect(() => {
    const term = new Terminal({
      cursorBlink: true,
      cursorStyle: 'block',
      fontSize: 14,
      fontFamily: "'Consolas', 'Monaco', 'Courier New', monospace",
      theme: {
        background: '#000000',
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

    // --- ApiClient setup ---
    const client = new ApiClient();
    clientRef.current = client;

    const unsubStatus = client.onStatusChange((s) => {
      setStatus(s);
      if (s === 'connected') {
        writePrompt(term);
      } else if (s === 'disconnected') {
        term.writeln('\r\n\x1b[33mDisconnected. Reconnecting...\x1b[0m');
      } else if (s === 'error') {
        term.writeln('\r\n\x1b[31mConnection error. Retrying...\x1b[0m');
      }
    });

    // Start connecting
    client.connect().catch((err) => {
      term.writeln(`\r\n\x1b[31mFailed to connect: ${(err as Error).message}\x1b[0m`);
    });

    // --- Terminal keyboard input ---
    term.onData((data) => {
      const code = data.charCodeAt(0);

      // Handle special keys
      if (data === '\r') {
        // Enter
        const line = currentLineRef.current.trim();
        term.writeln('');
        if (line.length > 0) {
          term.writeln(`\r\x1b[90mSending: ${line}\x1b[0m`);
          executeCommand(client, term, line);
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
      unsubStatus();
      client.disconnect();
      term.dispose();
    };
  }, []);

  return (
    <div
      style={{
        width: '100vw',
        height: '100vh',
        background: '#000',
        position: 'relative',
        overflow: 'hidden',
      }}
    >
      {status === 'connecting' && (
        <div
          style={{
            position: 'absolute',
            top: '50%',
            left: '50%',
            transform: 'translate(-50%, -50%)',
            color: '#00ff00',
            fontFamily: 'Consolas, monospace',
            fontSize: 16,
            zIndex: 10,
            background: '#000',
            padding: '20px',
          }}
        >
          Connecting to MikroLab simulator...
          <span className="loading-indicator" style={{ display: 'inline-block' }}>
            ⏳
          </span>
        </div>
      )}
      {status === 'error' && (
        <div
          style={{
            position: 'absolute',
            top: '50%',
            left: '50%',
            transform: 'translate(-50%, -50%)',
            color: '#ff4444',
            fontFamily: 'Consolas, monospace',
            fontSize: 16,
            zIndex: 10,
            background: '#000',
            padding: '20px',
          }}
        >
          ❌ Failed to connect to WebSocket server.
          <br />
          Make sure the backend is running on port 8080.
        </div>
      )}
      <div ref={terminalRef} style={{ width: '100%', height: '100%' }} />
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
 * Send a command to the API client and display the result.
 */
async function executeCommand(
  client: ApiClient,
  term: InstanceType<typeof Terminal>,
  line: string,
) {
  try {
    const resp = await client.sendCommand(line);
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