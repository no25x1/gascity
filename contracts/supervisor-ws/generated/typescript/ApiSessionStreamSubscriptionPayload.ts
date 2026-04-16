
interface ApiSessionStreamSubscriptionPayload {
  /**
   * Resume from this cursor
   */
  after_cursor?: string;
  /**
   * Stream format: 'text', 'raw', 'jsonl'
   */
  format?: string;
  /**
   * Must be 'session.stream'
   */
  kind?: string;
  /**
   * Session ID or session name
   */
  target?: string;
  /**
   * Most recent N turns (0=all)
   */
  turns?: number;
}
export { ApiSessionStreamSubscriptionPayload };