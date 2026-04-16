import {ApiEventsStreamPayload} from './ApiEventsStreamPayload';
interface ApiEventsStreamEventEnvelope {
  /**
   * Resume cursor for reconnection
   */
  cursor?: string;
  /**
   * Event type (e.g. 'bead.created')
   */
  event_type?: string;
  /**
   * Event sequence number
   */
  index?: number;
  payload?: ApiEventsStreamPayload;
  /**
   * Subscription that produced this event
   */
  subscription_id?: string;
  /**
   * Must be 'event'
   */
  type?: string;
}
export { ApiEventsStreamEventEnvelope };