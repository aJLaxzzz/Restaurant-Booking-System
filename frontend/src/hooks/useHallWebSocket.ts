import { useEffect, useRef, useState } from 'react';

export function useHallWebSocket(hallId: string | undefined, onMessage: (data: unknown) => void) {
  const cb = useRef(onMessage);
  cb.current = onMessage;
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    if (!hallId) return;
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const ws = new WebSocket(`${proto}//${host}/ws/halls/${hallId}`);
    ws.onopen = () => setConnected(true);
    ws.onclose = () => setConnected(false);
    ws.onmessage = (ev) => {
      try {
        cb.current(JSON.parse(ev.data as string));
      } catch {
        /* ignore */
      }
    };
    return () => {
      ws.close();
    };
  }, [hallId]);

  return { connected };
}
