export type CreateSourceResponseData = {
	uid: string;
	project_id: string;
	mask_id: string;
	name: string;
	url: string;
	type: string;
	provider: string;
	is_disabled: boolean;
	verifier: {
		type: string;
		hmac: null;
		basic_auth: null;
		api_key: null;
	};
	custom_response: {
		body: string;
		content_type: string;
	};
	provider_config: null;
	forward_headers: null;
	pub_sub: {
		type: string;
		workers: number;
		sqs: null;
		google: null;
		kafka: null;
		amqp: null;
	};
	idempotency_keys: null;
	body_function: null;
	header_function: null;
	created_at: string;
	updated_at: string;
	deleted_at: string | null;
};

export type CreateSourceBody = {
	name: string;
	is_disabled: boolean;
	type: string;
	body_function: null;
	header_function: null;
	custom_response: {
		body: string;
		content_type: string;
	};
	idempotency_keys: null;
	verifier: {
		type: 'hmac';
		hmac: {
			encoding: 'base64';
			hash: 'SHA256';
			header: 'X-Twitter-Webhooks-Signature';
			secret: 'twitter source';
		};
	};
	pub_sub: {
		type: string;
		workers: null;
		google: {
			service_account: string;
			subscription_id: string;
			project_id: string;
		};
		sqs: {
			queue_name: string;
			access_key_id: string;
			secret_key: string;
			default_region: string;
		};
		amqp: {
			schema: string;
			host: string;
			port: string;
			queue: string;
			deadLetterExchange: null;
			vhost: string;
			auth: {
				user: null;
				password: null;
			};
			bindExchange: {
				exchange: null;
				routingKey: string;
			};
		};
		kafka: {
			brokers: null;
			consumer_group_id: null;
			topic_name: null;
		};
	};
	provider: 'twitter' | 'github' | 'shopify' | '';
};

export interface SOURCE {
	created_at: Date;
	deleted_at: number;
	group_id: string;
	is_disabled: boolean;
	idempotency_keys: string[];
	mask_id: string;
	name: string;
	type: string;
	uid: string;
	updated_at: number;
	url: string;
	provider: 'github' | 'twitter' | 'shopify';
	provider_config?: { twitter: { crc_verified_at: Date } };
	"custom_response": {
		"body": string,
		"content_type": string
	},
	verifier: {
		api_key: {
			header_name: string;
			header_value: string;
		};
		basic_auth: {
			password: string;
			username: string;
		};
		hmac: {
			encoding: 'base64' | 'hex' | '';
			hash: 'SHA256' | 'SHA512' | '';
			header: string;
			secret: string;
		};
		type: 'hmac' | 'api_key' | 'basic_auth';
	};
	pub_sub: {
		sqs: {
			access_key_id: string;
			default_region: string;
			queue_name: string;
			secret_key: string;
		};
		amqp: {
			schema: string;
			host: string;
			port: string;
			queueName: string;
			deadLetterExchange: string;
			auth: {
				user: string;
				password: string;
			};
			bindExchange: {
				exchange: string;
				routingKey: string;
			};
		};
		kafka: {
			brokers: string[];
			consumer_group_id: string;
			topic_name: string;
			auth: {
				type: string;
				tls: boolean;
				username: string;
				password: string;
				hash: string;
			};
		};
		type: string;
		workers: number;
	};
}
