import {ExtmsgConversationRef} from './ExtmsgConversationRef';
interface ApiExtmsgBindRequest {
  conversation?: ExtmsgConversationRef;
  metadata?: Map<string, string>;
  session_id?: string;
}
export { ApiExtmsgBindRequest };