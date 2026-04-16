
interface ApiSocketSessionRespondPayload {
  action?: string;
  id?: string;
  metadata?: Map<string, string>;
  request_id?: string;
  text?: string;
}
export { ApiSocketSessionRespondPayload };