import { APP } from './app.model';
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
	endpoint_metadata: {
		target_url: string;
		secret: string;
	};
	app_metadata?: APP;
	alert_config?: { count: number; threshold: string };
	retry_config?: { type: string; retry_count: number };
	source_metadata: SOURCE;
	filter_config: { event_types: string[] };
	active_menu?: boolean;
}
