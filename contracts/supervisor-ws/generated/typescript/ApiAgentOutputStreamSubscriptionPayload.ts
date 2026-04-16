
interface ApiAgentOutputStreamSubscriptionPayload {
  /**
   * Resume from this cursor
   */
  after_cursor?: string;
  /**
   * Must be 'agent.output.stream'
   */
  kind?: string;
  /**
   * Agent name
   */
  target?: string;
}
export { ApiAgentOutputStreamSubscriptionPayload };