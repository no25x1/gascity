import {ExtmsgConversationRef} from './ExtmsgConversationRef';
interface ApiExtmsgTranscriptAckRequest {
  conversation?: ExtmsgConversationRef;
  sequence?: number;
  session_id?: string;
}
export { ApiExtmsgTranscriptAckRequest };