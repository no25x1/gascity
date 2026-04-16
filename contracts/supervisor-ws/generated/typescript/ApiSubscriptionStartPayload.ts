
interface ApiSubscriptionStartPayload {
  /**
   * Resume from this cursor
   */
  after_cursor?: string;
  /**
   * Resume from this event sequence
   */
  after_seq?: number;
  /**
   * Stream format: 'text', 'raw', 'jsonl'
   */
  format?: string;
  /**
   * Subscription type: 'events.stream', 'session.stream', or 'agent.output.stream'
   */
  kind?: string;
  /**
   * Stream target identifier (session ID/name or agent name)
   */
  target?: string;
  /**
   * Most recent N turns (0=all)
   */
  turns?: number;
}
export { ApiSubscriptionStartPayload };