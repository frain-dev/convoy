import {
	classifyEventLogsFetchError,
	eventLogsNetworkCopy,
	eventLogsSearchTimeoutCopy,
	eventLogsServerCopy
} from './event-logs-fetch-error';

describe('classifyEventLogsFetchError', () => {
	it('classifies timeout signals as a search timeout', () => {
		expect(classifyEventLogsFetchError('This request took too long. Try again.')).toEqual({
			searchTimedOut: true,
			fetchErrorMessage: eventLogsSearchTimeoutCopy
		});
		expect(classifyEventLogsFetchError('Search took too long')).toEqual({
			searchTimedOut: true,
			fetchErrorMessage: eventLogsSearchTimeoutCopy
		});
		expect(classifyEventLogsFetchError('504')).toEqual({
			searchTimedOut: true,
			fetchErrorMessage: eventLogsSearchTimeoutCopy
		});
	});

	it('classifies an unreachable server as a connection error', () => {
		expect(classifyEventLogsFetchError('Could not reach the server. Try again.')).toEqual({
			searchTimedOut: false,
			fetchErrorMessage: eventLogsNetworkCopy
		});
	});

	it('does not treat a 500 as a dropped connection', () => {
		expect(classifyEventLogsFetchError('Request failed with status code 500')).toEqual({
			searchTimedOut: false,
			fetchErrorMessage: eventLogsServerCopy
		});
		expect(classifyEventLogsFetchError('cannot start subtransactions during a parallel operation')).toEqual({
			searchTimedOut: false,
			fetchErrorMessage: eventLogsServerCopy
		});
	});
});
