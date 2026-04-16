import {BeadsDep} from './BeadsDep';
interface BeadsBead {
  assignee?: string;
  created_at?: string;
  dependencies?: BeadsDep[];
  description?: string;
  from?: string;
  id?: string;
  issue_type?: string;
  labels?: string[];
  metadata?: Map<string, string>;
  needs?: string[];
  parent?: string;
  priority?: number | null;
  ref?: string;
  status?: string;
  title?: string;
}
export { BeadsBead };