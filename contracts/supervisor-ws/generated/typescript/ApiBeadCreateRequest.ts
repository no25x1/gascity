
interface ApiBeadCreateRequest {
  assignee?: string;
  description?: string;
  labels?: string[] | null;
  priority?: number | null;
  rig?: string;
  title?: string;
  type?: string;
}
export { ApiBeadCreateRequest };