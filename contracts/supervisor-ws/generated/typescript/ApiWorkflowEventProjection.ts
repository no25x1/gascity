import {ApiWorkflowBeadResponse} from './ApiWorkflowBeadResponse';
interface ApiWorkflowEventProjection {
  attempt_summary?: Map<string, any>;
  bead?: ApiWorkflowBeadResponse;
  changed_fields?: string[] | null;
  event_seq?: number;
  event_ts?: string;
  event_type?: string;
  logical_node_id?: string;
  requires_resync?: boolean;
  root_bead_id?: string;
  root_store_ref?: string;
  scope_kind?: string;
  scope_ref?: string;
  type?: string;
  watch_generation?: string;
  workflow_id?: string;
  workflow_seq?: number;
}
export { ApiWorkflowEventProjection };