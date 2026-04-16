
interface ApiOrderResponse {
  capture_output?: boolean;
  check?: string;
  description?: string;
  enabled?: boolean;
  exec?: string;
  formula?: string;
  gate?: string;
  interval?: string;
  name?: string;
  on?: string;
  pool?: string;
  rig?: string;
  schedule?: string;
  scoped_name?: string;
  timeout?: string;
  timeout_ms?: number;
  type?: string;
}
export { ApiOrderResponse };