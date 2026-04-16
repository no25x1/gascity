import {ExtmsgConversationRef} from './ExtmsgConversationRef';
interface ApiExtmsgGroupEnsureRequest {
  default_handle?: string;
  metadata?: Map<string, string>;
  mode?: string;
  root_conversation?: ExtmsgConversationRef;
}
export { ApiExtmsgGroupEnsureRequest };