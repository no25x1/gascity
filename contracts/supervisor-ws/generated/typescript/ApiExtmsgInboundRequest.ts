import {ExtmsgExternalInboundMessage} from './ExtmsgExternalInboundMessage';
interface ApiExtmsgInboundRequest {
  account_id?: string;
  message?: ExtmsgExternalInboundMessage;
  payload?: string;
  provider?: string;
}
export { ApiExtmsgInboundRequest };