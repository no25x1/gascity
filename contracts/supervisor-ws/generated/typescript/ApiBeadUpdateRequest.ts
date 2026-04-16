
interface ApiBeadUpdateRequest {
  assignee?: string | null;
  description?: string | null;
  id?: string;
  labels?: string[];
  metadata?: Map<string, string>;
  priority?: Map<string, any>;
  remove_labels?: string[];
  status?: string | null;
  title?: string | null;
  type?: string | null;
}
export { ApiBeadUpdateRequest };