export interface EVENT {
	created_at: Date;
	provider_id: string;
	status: string;
	uid: string;
	updated_at: Date;
	app_id: string;
	event_type: string;
	metadata: {
		interval_seconds: number;
		next_send_time: Date;
		num_trials: number;
		retry_limit: number;
		strategy: string;
	};
}
