export interface GROUP {
	uid: string;
	name: string;
	logo_url: string;
	config: {
		Strategy: {
			type: string;
			default: {
				intervalSeconds: number;
				retryLimit: number;
			};
		};
		Signature: {
			header: string;
			hash: string;
		};
		DisableEndpoint: boolean;
	};
	statistics: {
		messages_sent: number;
		total_apps: number;
	};
	created_at: Date;
	updated_at: Date;
	showDropdown: boolean;
}
