import { differenceInCalendarDays, format, subDays } from 'date-fns';

export interface DeliveryDateRange {
	startDate: string;
	endDate: string;
}

export function deliveryRangeDate(value: string | Date): Date {
	return new Date(typeof value === 'string' ? `${value}Z` : value);
}

// One explicit range for chart, cards, list and count. Never let omitted table
// dates silently select the API's different default window.
export function resolveDeliveryDateRange(params: { startDate?: unknown; endDate?: unknown } = {}, now = new Date(), remembered?: DeliveryDateRange): DeliveryDateRange {
	const start = new Date(now);
	start.setDate(start.getDate() - 30);
	start.setHours(0, 0, 0, 0);
	const end = new Date(now);
	end.setHours(23, 59, 59, 0);
	const fallback = remembered || {
		startDate: start.toISOString().slice(0, 19),
		endDate: end.toISOString().slice(0, 19)
	};
	const range = { startDate: params.startDate || fallback.startDate, endDate: params.endDate || fallback.endDate };
	const valid = (value: unknown): value is string => {
		if (typeof value !== 'string' || value.length !== 19) return false;
		const date = deliveryRangeDate(value);
		return !Number.isNaN(date.getTime()) && date.toISOString().slice(0, 19) === value;
	};
	if (!valid(range.startDate) || !valid(range.endDate) || range.startDate > range.endDate) return fallback;
	return { startDate: range.startDate, endDate: range.endDate };
}

// Save offsets rather than absolute dates: a fresh visit advances the window,
// while an explicit URL still represents the exact shared/history range.
export function rememberedDeliveryDateRange(scope?: string, now = new Date()): DeliveryDateRange {
	const fallback = resolveDeliveryDateRange({}, now);
	if (!scope) return fallback;
	try {
		const saved = JSON.parse(localStorage.getItem(`CONVOY_DELIVERY_RANGE_V1:${scope}`) || 'null');
		const validOffset = (value: unknown): value is number => Number.isInteger(value) && (value as number) >= 0 && (value as number) <= 365000;
		const validTime = (value: unknown): value is string => typeof value === 'string' && /^(?:[01]\d|2[0-3]):[0-5]\d:[0-5]\d$/.test(value);
		if (!saved || !validOffset(saved.startDaysAgo) || !validOffset(saved.endDaysAgo) || saved.startDaysAgo < saved.endDaysAgo || !validTime(saved.startTime) || !validTime(saved.endTime)) return fallback;
		return resolveDeliveryDateRange({
			startDate: new Date(`${format(subDays(now, saved.startDaysAgo), 'yyyy-MM-dd')}T${saved.startTime}`).toISOString().slice(0, 19),
			endDate: new Date(`${format(subDays(now, saved.endDaysAgo), 'yyyy-MM-dd')}T${saved.endTime}`).toISOString().slice(0, 19)
		}, now);
	} catch {
		return fallback;
	}
}

export function rememberDeliveryDateRange(scope: string | undefined, range?: DeliveryDateRange, now = new Date()): void {
	if (!scope) return;
	try {
		const key = `CONVOY_DELIVERY_RANGE_V1:${scope}`;
		if (!range) {
			localStorage.removeItem(key);
			return;
		}
		const start = deliveryRangeDate(range.startDate);
		const end = deliveryRangeDate(range.endDate);
		localStorage.setItem(key, JSON.stringify({
			startDaysAgo: differenceInCalendarDays(now, start),
			endDaysAgo: differenceInCalendarDays(now, end),
			startTime: format(start, 'HH:mm:ss'),
			endTime: format(end, 'HH:mm:ss')
		}));
	} catch {
		// Storage can be unavailable; date filtering must still work.
	}
}
