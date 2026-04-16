
interface MailMessage {
  body?: string;
  cc?: string[];
  created_at?: string;
  from?: string;
  id?: string;
  priority?: number;
  read?: boolean;
  reply_to?: string;
  rig?: string;
  subject?: string;
  thread_id?: string;
  to?: string;
}
export { MailMessage };