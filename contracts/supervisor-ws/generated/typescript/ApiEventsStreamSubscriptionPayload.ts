
interface ApiEventsStreamSubscriptionPayload {
  /**
   * Resume from this cursor
   */
  after_cursor?: string;
  /**
   * Resume from this event sequence
   */
  after_seq?: number;
  /**
   * Must be 'events.stream'
   */
  kind?: string;
}
export { ApiEventsStreamSubscriptionPayload };