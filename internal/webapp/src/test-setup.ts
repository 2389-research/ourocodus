// Mock WebSocket
class MockWebSocket {
  public readyState: number;
  public onopen: ((event: Event) => void) | null;
  public onmessage: ((event: MessageEvent) => void) | null;
  public onerror: ((event: Event) => void) | null;
  public onclose: ((event: CloseEvent) => void) | null;
  public url: string;

  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;

  constructor(url: string) {
    this.url = url;
    this.readyState = MockWebSocket.CONNECTING;
    this.onopen = null;
    this.onmessage = null;
    this.onerror = null;
    this.onclose = null;
  }

  send(data: string): void {
    if (this.readyState !== MockWebSocket.OPEN) {
      throw new Error('WebSocket is not open');
    }
    // Mock implementation - tests can spy on this
  }

  close(code?: number, reason?: string): void {
    this.readyState = MockWebSocket.CLOSED;
    if (this.onclose) {
      this.onclose(new CloseEvent('close', { code, reason }));
    }
  }

  // Helper for tests to simulate opening connection
  _simulateOpen(): void {
    this.readyState = MockWebSocket.OPEN;
    if (this.onopen) {
      this.onopen(new Event('open'));
    }
  }

  // Helper for tests to simulate receiving messages
  _simulateMessage(data: string): void {
    if (this.onmessage) {
      this.onmessage(new MessageEvent('message', { data }));
    }
  }

  // Helper for tests to simulate errors
  _simulateError(): void {
    if (this.onerror) {
      this.onerror(new Event('error'));
    }
  }
}

// Mock localStorage
const localStorageMock = {
  getItem: (key: string) => null,
  setItem: (key: string, value: string) => {},
  removeItem: (key: string) => {},
  clear: () => {},
  key: (index: number) => null,
  length: 0,
};

(global as any).localStorage = localStorageMock;

// Install WebSocket mock globally
(global as any).WebSocket = MockWebSocket;

// Mock navigator.serviceWorker (if it doesn't exist)
if (typeof (global as any).navigator === 'undefined') {
  (global as any).navigator = {
    serviceWorker: undefined,
  };
} else if (!(global as any).navigator.serviceWorker) {
  Object.defineProperty((global as any).navigator, 'serviceWorker', {
    value: undefined,
    writable: true,
    configurable: true,
  });
}

// Mock window.app
if (typeof (global as any).window === 'undefined') {
  (global as any).window = global;
}
