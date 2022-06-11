import { SOURCE } from './group.model';

export interface SUBSCRIPTION {
	created_at: string;
	endpoint: string;
	name: string;
	source: SOURCE;
	status: string;
	type: 'outgoing' | 'incoming';
	uid: string;
	updated_at: string;
	endpoint_metadata?: {
		target_url: string;
	};
	app_metadata?: {
		name: string;
	};
	alert_config?: { count: number; threshold: string };
	retry_config?: { type: string; retry_count: number };
	filter_config?: {
		event_types: string[];
	};
}
