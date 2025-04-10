import type { ENDPOINT } from './endpoint.model';
import type { SOURCE } from './source';

export interface Event {
	uid: string;
	event_type: string;
	project_id: string;
	endpoints: null | Array<ENDPOINT>;
	headers: null;
	source_metadata: SOURCE;
	url_query_params: '';
	idempotency_key: '';
	is_duplicate_event: false;
	data: Record<string, unknown>;
	raw: string;
	status: string;
	acknowledged_at: string;
	created_at: string;
	updated_at: string;
	deleted_at: null | string;
}
