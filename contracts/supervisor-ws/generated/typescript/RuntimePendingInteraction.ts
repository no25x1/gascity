
interface RuntimePendingInteraction {
  kind?: string;
  metadata?: Map<string, string>;
  options?: string[];
  prompt?: string;
  request_id?: string;
}
export { RuntimePendingInteraction };