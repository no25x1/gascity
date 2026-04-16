import {ExtmsgExternalActor} from './ExtmsgExternalActor';
import {ExtmsgExternalAttachment} from './ExtmsgExternalAttachment';
import {ExtmsgConversationRef} from './ExtmsgConversationRef';
interface ExtmsgExternalInboundMessage {
  actor?: ExtmsgExternalActor;
  attachments?: ExtmsgExternalAttachment[];
  conversation?: ExtmsgConversationRef;
  dedup_key?: string;
  explicit_target?: string;
  provider_message_id?: string;
  received_at?: string;
  reply_to_message_id?: string;
  text?: string;
}
export { ExtmsgExternalInboundMessage };