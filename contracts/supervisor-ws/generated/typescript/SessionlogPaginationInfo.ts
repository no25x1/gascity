
interface SessionlogPaginationInfo {
  has_older_messages?: boolean;
  returned_message_count?: number;
  total_compactions?: number;
  total_message_count?: number;
  truncated_before_message?: string;
}
export { SessionlogPaginationInfo };