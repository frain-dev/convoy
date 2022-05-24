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
	created_at: Date;
	updated_at: Date;
}

export interface SOURCE {
	created_at: number;
	deleted_at: number;
	group_id: string;
	is_disabled: boolean;
	mask_id: string;
	name: string;
	type: string;
	uid: string;
	updated_at: number;
	url: string;
	verifier: {
		api_key: {
			header: string;
			key: string;
		};
		basic_auth: {
			password: string;
			username: string;
		};
		hmac: {
			encoding: string;
			hash: string;
			header: string;
			secret: string;
		};
		type: string;
	};
}
