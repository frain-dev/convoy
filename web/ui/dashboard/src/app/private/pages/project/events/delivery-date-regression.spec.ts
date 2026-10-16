import { FormBuilder } from '@angular/forms';
import { BehaviorSubject } from 'rxjs';
import { EventsComponent } from './events.component';
import { EventDeliveriesComponent as PortalPage } from 'src/app/portal/event-deliveries/event-deliveries.component';
import { EventDeliveriesComponent as DeliveryTable } from './event-deliveries/event-deliveries.component';
import { rememberedDeliveryDateRange, rememberDeliveryDateRange, resolveDeliveryDateRange } from './delivery-date-range';
import { GeneralService } from 'src/app/services/general/general.service';

const savedRange = { startDate: '2026-08-01T10:15:00', endDate: '2026-08-02T12:30:00' };
const selectedRange = { startDate: '2026-09-01T00:00:00', endDate: '2026-09-02T23:59:59' };
const response = (count: number) => ({ data: { events_sent: count, apps: 1, event_data: [] } });

describe('delivery date range', () => {
	it('uses explicit day boundaries for the default range', () => {
		expect(resolveDeliveryDateRange({}, new Date(2026, 9, 12, 19, 0))).toEqual({ startDate: new Date(2026, 8, 12).toISOString().slice(0, 19), endDate: new Date(2026, 9, 12, 23, 59, 59).toISOString().slice(0, 19) });
	});
	it('preserves saved time filters and rejects malformed or reversed URL ranges', () => {
		expect(resolveDeliveryDateRange(savedRange)).toEqual(savedRange);
		const now = new Date(2026, 9, 12);
		const fallback = resolveDeliveryDateRange({}, now);
		for (const params of [{ startDate: 'bad' }, { startDate: ['2026-09-01T00:00:00'] }, { startDate: '2026-02-31T00:00:00' }, { startDate: savedRange.endDate, endDate: savedRange.startDate }]) {
			expect(resolveDeliveryDateRange(params, now)).toEqual(fallback);
		}
	});
});

describe('remembered rolling delivery range', () => {
	const scope = 'date-regression-user:project-a';
	const otherScope = 'date-regression-user:project-b';
	afterEach(() => {
		rememberDeliveryDateRange(scope);
		rememberDeliveryDateRange(otherScope);
	});
	it('advances a saved window across days while preserving time filters', () => {
		rememberDeliveryDateRange(scope, { startDate: '2026-10-05T10:15:00', endDate: '2026-10-12T12:30:59' }, new Date(2026, 9, 12));
		expect(rememberedDeliveryDateRange(scope, new Date(2026, 9, 15))).toEqual({ startDate: '2026-10-08T10:15:00', endDate: '2026-10-15T12:30:59' });
	});
	it('keeps projects separate, lets URL dates override the preference and resets on clear', () => {
		const now = new Date(2026, 9, 12);
		rememberDeliveryDateRange(scope, { startDate: '2026-10-11T00:00:00', endDate: '2026-10-11T23:59:59' }, now);
		expect(rememberedDeliveryDateRange(otherScope, now)).toEqual(resolveDeliveryDateRange({}, now));
		expect(resolveDeliveryDateRange(savedRange, now, rememberedDeliveryDateRange(scope, now))).toEqual(savedRange);
		rememberDeliveryDateRange(scope);
		expect(rememberedDeliveryDateRange(scope, now)).toEqual(resolveDeliveryDateRange({}, now));
	});
	it('falls back when stored values are corrupt or storage is unavailable', () => {
		const now = new Date(2026, 9, 12);
		localStorage.setItem(`CONVOY_DELIVERY_RANGE_V1:${scope}`, JSON.stringify({ startDaysAgo: -1, endDaysAgo: 0, startTime: 'bad', endTime: '23:59:59' }));
		expect(rememberedDeliveryDateRange(scope, now)).toEqual(resolveDeliveryDateRange({}, now));
		spyOn(Storage.prototype, 'getItem').and.throwError('blocked');
		expect(rememberedDeliveryDateRange(scope, now)).toEqual(resolveDeliveryDateRange({}, now));
		spyOn(Storage.prototype, 'setItem').and.throwError('blocked');
		expect(() => rememberDeliveryDateRange(scope, savedRange, now)).not.toThrow();
	});
});

for (const portal of [false, true]) {
	describe(`${portal ? 'Portal' : 'Project'} delivery page dates`, () => {
		let page: EventsComponent | PortalPage;
		let service: any;
		let params: BehaviorSubject<any>;
		let table: any;
		beforeEach(() => {
			params = new BehaviorSubject(savedRange);
			const route: any = { snapshot: { queryParams: savedRange }, queryParams: params };
			service = {
				dashboardSummary: jasmine.createSpy().and.resolveTo(response(1)),
				getSummaryDeliveryCounts: jasmine.createSpy().and.resolveTo({ success: 1, failure: 0 })
			};
			const privateService: any = {
				getProjectDetails: { uid: 'date-regression-project', type: 'outgoing' },
				getUserProfile: { uid: 'date-regression-user' },
				getProjectStat: jasmine.createSpy().and.resolveTo({ data: { subscriptions_exist: true, events_exist: true } }),
				getEndpoints: jasmine.createSpy().and.resolveTo({ data: { content: [], pagination: {} } })
			};
			page = portal
				? new PortalPage(new FormBuilder(), service, privateService, { setLicenses() {} } as any, {} as any, route)
				: new EventsComponent(new FormBuilder(), service, privateService, {} as any, route);
			table = { applyDateFilter: jasmine.createSpy(), restoreFiltersFromURL: jasmine.createSpy() };
			page.eventDeliveriesTable = table;
		});
		afterEach(() => {
			page.ngOnDestroy();
			rememberDeliveryDateRange('date-regression-user:date-regression-project');
		});

		it('restores URL dates for the initial chart and card requests', async () => {
			await page.ngOnInit();
			expect(page.statsDateRangeForRequest).toEqual(savedRange);
			expect(service.dashboardSummary).toHaveBeenCalledWith({ ...savedRange, type: 'daily' });
			expect(service.getSummaryDeliveryCounts).toHaveBeenCalledWith({ ...savedRange, endpointId: undefined });
		});
		it('applies the same manual time filter to chart, cards and table', () => {
			page.getSelectedDateRange(selectedRange as any);
			expect(service.dashboardSummary).toHaveBeenCalledWith({ ...selectedRange, type: 'daily' });
			expect(service.getSummaryDeliveryCounts).toHaveBeenCalledWith({ ...selectedRange, endpointId: undefined });
			expect(table.applyDateFilter).toHaveBeenCalledWith(selectedRange);
			if (!portal) expect(rememberedDeliveryDateRange('date-regression-user:date-regression-project')).toEqual(selectedRange);
		});
		it('clears dates to one explicit default for every request', () => {
			page.getSelectedDateRange();
			const range = page.statsDateRangeForRequest;
			expect(range).toEqual(resolveDeliveryDateRange());
			expect(table.applyDateFilter).toHaveBeenCalledWith(range);
			expect(service.dashboardSummary).toHaveBeenCalledWith({ ...range, type: 'daily' });
			if (!portal) expect(rememberedDeliveryDateRange('date-regression-user:date-regression-project')).toEqual(resolveDeliveryDateRange());
		});
		if (!portal) it('uses the remembered project range on a fresh visit without URL dates', async () => {
			page.getSelectedDateRange(selectedRange as any);
			await page.ngOnInit();
			params.next({});
			expect(page.statsDateRangeForRequest).toEqual(selectedRange);
			expect(table.restoreFiltersFromURL).toHaveBeenCalledWith(selectedRange);
		});
		it('restores history dates without adding another browser history entry', async () => {
			await page.ngOnInit();
			params.next(selectedRange);
			expect(page.statsDateRangeForRequest).toEqual(selectedRange);
			expect(table.restoreFiltersFromURL).toHaveBeenCalledWith(selectedRange);
		});
		it('restores table-only history filters even when dates stay the same', async () => {
			await page.ngOnInit();
			params.next({ ...savedRange, query: 'history search' });
			expect(table.restoreFiltersFromURL).toHaveBeenCalledWith({ ...savedRange, query: 'history search' });
		});
		it('ignores an old chart response after a newer range finishes', async () => {
			let finishOld!: (value: any) => void;
			service.dashboardSummary.and.returnValues(new Promise(resolve => finishOld = resolve), Promise.resolve(response(2)));
			const old = page.fetchDashboardData();
			page.statsDateRange.patchValue(selectedRange);
			await page.fetchDashboardData();
			finishOld(response(100));
			await old;
			expect(page.dashboardData.events_sent).toBe(2);
		});
	});
}

describe('delivery table range', () => {
	let table: DeliveryTable;
	let service: any;
	let url: any;
	beforeEach(() => {
		const route: any = { snapshot: { queryParams: { ...savedRange, token: 'portal-token', query: 'old search' } } };
		service = { getEventDeliveries: jasmine.createSpy().and.resolveTo({ data: { content: [], pagination: {} } }) };
		url = { go: jasmine.createSpy(), replaceState: jasmine.createSpy() };
		const general = Object.create(GeneralService.prototype);
		general.route = route;
		url.path = () => '/portal/event-deliveries';
		general._location = url;
		table = new DeliveryTable(general, service, route, {} as any, {} as any, url);
		table.dateRange = selectedRange;
		spyOn(table, 'refreshTotalCount');
		spyOn(table, 'getEventTypesForFilter');
		spyOn(table, 'getEndpointsForFilter');
		spyOn(table, 'checkIfTailModeIsEnabled').and.returnValue(false);
		spyOn(table, 'setEventDeliveriesContent').and.returnValue([]);
	});
	afterEach(() => table.ngOnDestroy());
	it('uses the parent range even when the URL contains different dates', () => {
		table.ngOnInit();
		expect(service.getEventDeliveries).toHaveBeenCalledWith(jasmine.objectContaining(selectedRange));
		expect(url.replaceState).toHaveBeenCalledWith('/portal/event-deliveries', jasmine.stringMatching('startDate=2026-09-01T00%3A00%3A00'));
	});
	it('clears table filters while retaining the current page dates and portal token', () => {
		table.ngOnInit();
		table.applyDateFilter(savedRange);
		table.clearAllFilters();
		expect(table.queryParams).toEqual({ ...savedRange, token: 'portal-token' } as any);
		expect(service.getEventDeliveries).toHaveBeenCalledWith(jasmine.objectContaining(savedRange));
		expect(url.go.calls.mostRecent().args[0]).toContain('startDate=2026-08-01T10%3A15%3A00');
	});
	it('ignores a late list response after changing the range', async () => {
		let finishOld!: (value: any) => void;
		service.getEventDeliveries.and.returnValues(new Promise(resolve => finishOld = resolve), Promise.resolve({ data: { content: [], pagination: { per_page: 2 } } }));
		const old = table.getEventDeliveries(savedRange);
		await table.getEventDeliveries(selectedRange);
		finishOld({ data: { content: [], pagination: { per_page: 100 } } });
		await old;
		expect(table.eventDeliveries?.pagination?.per_page).toBe(2);
	});
	it('does not revive old URL dates or search after clearing and selecting another filter', () => {
		table.ngOnInit();
		table.clearAllFilters();
		table.updateEndpointFilter({ uid: 'endpoint-2' } as any);
		expect(table.queryParams.startDate).toBe(selectedRange.startDate);
		expect(table.queryParams.query).toBeUndefined();
	});
	it('replaces all table filters from a history entry without writing a new URL', () => {
		table.ngOnInit();
		table.restoreFiltersFromURL({ ...selectedRange, status: '["Failure"]', query: 'history search' });
		expect(table.searchString).toBe('history search');
		expect(table.statusFilter).toEqual(['Failure']);
		table.restoreFiltersFromURL(savedRange);
		expect(table.searchString).toBe('');
		expect(table.statusFilter).toEqual([]);
		expect(table.queryParams).toEqual(savedRange);
		expect(url.go).not.toHaveBeenCalled();
	});
});
