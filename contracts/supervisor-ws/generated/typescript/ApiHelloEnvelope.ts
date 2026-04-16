
interface ApiHelloEnvelope {
  /**
   * Sorted list of supported action names
   */
  capabilities?: string[] | null;
  /**
   * Protocol version (e.g. 'gc.v1alpha1')
   */
  protocol?: string;
  /**
   * True if mutations are disabled
   */
  read_only?: boolean;
  /**
   * 'city' or 'supervisor'
   */
  server_role?: string;
  /**
   * Supported subscription types
   */
  subscription_kinds?: string[];
  /**
   * Must be 'hello'
   */
  type?: string;
}
export { ApiHelloEnvelope };