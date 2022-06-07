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
}
