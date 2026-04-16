
interface ApiSocketEventsListPayload {
  actor?: string;
  cursor?: string;
  limit?: number | null;
  since?: string;
  type?: string;
}
export { ApiSocketEventsListPayload };