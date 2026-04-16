import {ApiStreamActivityPayload} from './ApiStreamActivityPayload';
interface ApiSessionStreamActivityEventEnvelope {
  /**
   * Must be 'activity'
   */
  event_type?: string;
  /**
   * Event sequence number
   */
  index?: number;
  payload?: ApiStreamActivityPayload;
  /**
   * Subscription that produced this event
   */
  subscription_id?: string;
  /**
   * Must be 'event'
   */
  type?: string;
}
export { ApiSessionStreamActivityEventEnvelope };