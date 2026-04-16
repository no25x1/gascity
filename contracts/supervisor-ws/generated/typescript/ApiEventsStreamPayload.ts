import {ApiWorkflowEventProjection} from './ApiWorkflowEventProjection';
interface ApiEventsStreamPayload {
  actor?: string;
  city?: string;
  message?: string;
  payload?: any;
  seq?: number;
  subject?: string;
  ts?: string;
  type?: string;
  workflow?: ApiWorkflowEventProjection;
}
export { ApiEventsStreamPayload };