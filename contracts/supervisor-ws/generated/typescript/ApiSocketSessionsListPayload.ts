
interface ApiSocketSessionsListPayload {
  cursor?: string;
  limit?: number | null;
  peek?: boolean;
  state?: string;
  template?: string;
}
export { ApiSocketSessionsListPayload };