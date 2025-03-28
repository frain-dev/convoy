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
