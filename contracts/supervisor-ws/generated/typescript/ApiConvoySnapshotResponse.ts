import {BeadsBead} from './BeadsBead';
import {ApiConvoyProgressResponse} from './ApiConvoyProgressResponse';
interface ApiConvoySnapshotResponse {
  children?: BeadsBead[] | null;
  convoy?: BeadsBead;
  progress?: ApiConvoyProgressResponse;
}
export { ApiConvoySnapshotResponse };