export function getInitials(names: Array<string>) {
	const initials = names
		.filter(Boolean)
		.map(name => name[0])
		.join('');
	if (initials.length < 3) return initials;
	return initials[0] + initials[initials.length - 1];
}

export function truncateProjectName(name: string) {
	const formattedName = name.substring(0, 27);
	if (name > formattedName) return formattedName + '...';
	return formattedName;
}

/**
 * @param {string} date an ISO date string
 * @example
 * // returns 'Nov 3, 1994'
 * toMMM_DD_YYYY("1994-11-03T00:00:00Z")
 */
export function toMMMDDYYYY(date: string) {
	return new Intl.DateTimeFormat('en-US', { dateStyle: 'medium' }).format(
		new Date(date),
	);
}

export function groupItemsByDate<T>(
	items: Array<T & { created_at: string }>,
	sortOrder: 'desc' | 'asc' = 'desc',
) {
	const groupsObj = Object.groupBy(items, ({ created_at }) =>
		toMMMDDYYYY(created_at),
	);

	const sortedGroup = new Map<string, typeof items>();

	Object.keys(groupsObj)
		.sort((dateA, dateB) => {
			if (sortOrder == 'desc') {
				return Number(new Date(dateB)) - Number(new Date(dateA));
			}
			return Number(new Date(dateA)) - Number(new Date(dateB));
		})
		.reduce((acc, dateKey) => {
			return acc.set(dateKey, groupsObj[dateKey] as typeof items);
		}, sortedGroup);

	return sortedGroup;
}

export function stringToJson(str: string) {
	if (!str) return null;
	try {
		const jsonObject = JSON.parse(str);
		return jsonObject;
	} catch (error) {
		console.error(error);
		// returning `undefined` as err value because `undefined` is invalid JSON
		return undefined;
	}
}

export function transformSourceValueType(
	value: string,
	type: 'sourceType' | 'verifier' | 'pub_sub',
) {
	const sourceTypes = [
		{ value: 'http', viewValue: 'HTTP' },
		{ value: 'rest_api', viewValue: 'Rest API' },
		{ value: 'pub_sub', viewValue: 'Pub/Sub' },
		{ value: 'db_change_stream', viewValue: 'Database' },
	];
	const httpTypes = [
		{ value: 'hmac', viewValue: 'HMAC' },
		{ value: 'basic_auth', viewValue: 'Basic Auth' },
		{ value: 'api_key', viewValue: 'API Key' },
		{ value: 'noop', viewValue: 'None' },
	];

	const pubSubTypes = [
		{ value: 'google', viewValue: 'Google Pub/Sub' },
		{ value: 'sqs', viewValue: 'AWS SQS' },
		{ value: 'kafka', viewValue: 'Kafka' },
		{ value: 'amqp', viewValue: 'AMQP / RabbitMQ' },
	];

	if (type === 'sourceType') {
		return sourceTypes.find(source => source.value === value)?.viewValue || '-';
	}
	if (type === 'verifier') {
		return httpTypes.find(source => source.value === value)?.viewValue || '-';
	}
	if (type === 'pub_sub') {
		return pubSubTypes.find(source => source.value === value)?.viewValue || '-';
	}

	return '-';
}
