export interface FLIPT_API_RESPONSE {
	requestId: string;
	responses: FLIPT_RESPONSE[];
	requestDurationMillis: 0;
}

export interface FLIPT_RESPONSE {
	requestId: string;
	entityId: string;
	requestContext: { [key: string]: string[] };
	match: boolean;
	flagKey: string;
	segmentKey: string;
	timestamp: string;
	value: string;
	requestDurationMillis: 0;
	attachment: string;
}
