export const eventLogsSearchTimeoutCopy =
	'Search took too long. Narrow the date range, simplify your JSON filter, or try a shorter search term.';
export const eventLogsNetworkCopy = 'Check your connection and try again.';
export const eventLogsServerCopy = 'Try again.';

export function classifyEventLogsFetchError(error: unknown): { searchTimedOut: boolean; fetchErrorMessage: string } {
	const msg = typeof error === 'string' ? error : String((error as { message?: unknown } | undefined)?.message || error || '');
	// Timeout only from timeout signals. A 500 body is not a timeout and is not a dropped connection.
	if (/search took too long|took too long|504|timed out|\btimeout\b/i.test(msg)) {
		return { searchTimedOut: true, fetchErrorMessage: eventLogsSearchTimeoutCopy };
	}
	if (/reach the server/i.test(msg)) {
		return { searchTimedOut: false, fetchErrorMessage: eventLogsNetworkCopy };
	}
	return { searchTimedOut: false, fetchErrorMessage: eventLogsServerCopy };
}
