
interface ApiWatchParams {
  /**
   * Block until server index exceeds this value
   */
  index?: number;
  /**
   * Maximum wait duration (e.g. '30s')
   */
  wait?: string;
}
export { ApiWatchParams };