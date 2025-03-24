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
	provider: string;
	provider_config?: { twitter: { crc_verified_at: Date } };
	custom_response: {
		body: string;
		content_type: string;
	};
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
			encoding: string;
			hash: string;
			header: string;
			secret: string;
		};
		type: string;
	};
	pub_sub: {
		google: {
			service_account: string;
			subscription_id: string;
			project_id: string;
		};
		sqs: {
			access_key_id: string;
			default_region: string;
			queue_name: string;
			secret_key: string;
		};
		amqp: {
			schema: string,
			host: string,
			port: string,
			queueName: string,
			deadLetterExchange: string,
			auth: {
				user: string,
				password: string,
			},
			bindExchange: {
				exchange: string,
				routingKey: string,
			},
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
