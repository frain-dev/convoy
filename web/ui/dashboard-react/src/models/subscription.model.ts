import type { DEVICE } from './device';
import type { SOURCE } from './source';
import type { ENDPOINT } from './endpoint.model';

export interface SUBSCRIPTION {
	created_at: string;
	endpoint: string;
	name: string;
	function: string;
	source: SOURCE;
	status: string;
	type: 'outgoing' | 'incoming';
	uid: string;
	project_id: string;
	updated_at: string;
	endpoint_metadata?: ENDPOINT;
	alert_config?: { count: number; threshold: string };
	retry_config?: { type: string; retry_count: number; duration: number };
	source_metadata: SOURCE;
	filter_config: {
		event_types: string[];
		filter: { headers: Record<string, unknown>; body: Record<string, unknown> };
	};
	active_menu?: boolean;
	device_metadata: DEVICE;
}
