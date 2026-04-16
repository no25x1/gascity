import {RuntimePendingInteraction} from './RuntimePendingInteraction';
interface ApiSessionStreamPendingEventEnvelope {
  /**
   * Must be 'pending'
   */
  event_type?: string;
  /**
   * Event sequence number
   */
  index?: number;
  payload?: RuntimePendingInteraction;
  /**
   * Subscription that produced this event
   */
  subscription_id?: string;
  /**
   * Must be 'event'
   */
  type?: string;
}
export { ApiSessionStreamPendingEventEnvelope };