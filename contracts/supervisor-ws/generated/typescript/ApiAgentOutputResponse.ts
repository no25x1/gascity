import {SessionlogPaginationInfo} from './SessionlogPaginationInfo';
import {ApiOutputTurn} from './ApiOutputTurn';
interface ApiAgentOutputResponse {
  agent?: string;
  format?: string;
  pagination?: SessionlogPaginationInfo;
  turns?: ApiOutputTurn[] | null;
}
export { ApiAgentOutputResponse };