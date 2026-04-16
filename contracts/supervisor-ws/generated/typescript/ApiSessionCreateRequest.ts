
interface ApiSessionCreateRequest {
  alias?: string;
  async?: boolean;
  kind?: string;
  message?: string;
  name?: string;
  options?: Map<string, string>;
  project_id?: string;
  session_name?: string | null;
  title?: string;
}
export { ApiSessionCreateRequest };