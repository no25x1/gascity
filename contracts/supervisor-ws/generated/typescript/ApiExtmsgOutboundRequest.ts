import {ExtmsgConversationRef} from './ExtmsgConversationRef';
interface ApiExtmsgOutboundRequest {
  conversation?: ExtmsgConversationRef;
  idempotency_key?: string;
  reply_to_message_id?: string;
  session_id?: string;
  text?: string;
}
export { ApiExtmsgOutboundRequest };