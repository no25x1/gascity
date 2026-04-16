import {ApiSessionTranscriptResult} from './ApiSessionTranscriptResult';
interface ApiSessionStreamTurnEventEnvelope {
  /**
   * Resume cursor for reconnection
   */
  cursor?: string;
  /**
   * Must be 'turn'
   */
  event_type?: string;
  /**
   * Event sequence number
   */
  index?: number;
  payload?: ApiSessionTranscriptResult;
  /**
   * Subscription that produced this event
   */
  subscription_id?: string;
  /**
   * Must be 'event'
   */
  type?: string;
}
export { ApiSessionStreamTurnEventEnvelope };