import {BeadsBead} from './BeadsBead';
import {ApiWorkflowDepResponse} from './ApiWorkflowDepResponse';
interface ApiBeadGraphResponseJson {
  beads?: BeadsBead[] | null;
  deps?: ApiWorkflowDepResponse[] | null;
  root?: BeadsBead;
}
export { ApiBeadGraphResponseJson };