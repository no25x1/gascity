
interface ApiWorkflowBeadResponse {
  assignee?: string;
  attempt?: number | null;
  id?: string;
  kind?: string;
  logical_bead_id?: string;
  metadata?: Map<string, string> | null;
  scope_ref?: string;
  status?: string;
  step_ref?: string;
  title?: string;
}
export { ApiWorkflowBeadResponse };