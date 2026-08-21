import { Component, OnInit, ViewChild } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';
import { format } from 'date-fns';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { EventsService } from './events.service';
import { EVENT_DELIVERY } from 'src/app/models/event.model';
import { CHARTDATA, PAGINATION } from 'src/app/models/global.model';
import { PrivateService } from 'src/app/private/private.service';
import { Router } from '@angular/router';
import { SOURCE } from 'src/app/models/source.model';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { EventDeliveriesComponent } from './event-deliveries/event-deliveries.component';

@Component({
    selector: 'app-events',
    templateUrl: './events.component.html',
    styleUrls: ['./events.component.scss'],
    standalone: false
})
export class EventsComponent implements OnInit {
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	isloadingDashboardData: boolean = false;
	showFilterDropdown: boolean = false;
	selectedDateOption: string = '';
	dashboardFrequency: 'daily' | 'weekly' | 'monthly' | 'yearly' = 'daily';
	filterOptions: ['daily', 'weekly', 'monthly'] = ['daily', 'weekly', 'monthly'];
	dashboardData = { apps: 0, events_sent: 0 };
	eventDeliveries!: { pagination: PAGINATION; content: EVENT_DELIVERY[] };
	statsDateRange: FormGroup = this.formBuilder.group({
		startDate: [{ value: new Date(new Date().setDate(new Date().getDate() - 30)), disabled: true }],
		endDate: [{ value: new Date(), disabled: true }]
	});
	dateRangeValue?: {
		startDate: string | Date;
		endDate: string | Date;
	};
	hasEvents: boolean = false;
	chartData!: CHARTDATA[];
	showAddEventModal = false;
	lastestSource?: SOURCE;
	lastestEventDeliveries: EVENT_DELIVERY[] = [];
	eventDelTableHead: string[] = ['Status', 'Event Type', 'Event Time', 'Next Attempt'];
	eventDelievryIntervalTime: any;
	labelsDateFormat!: string;
	isProjectConfigurationComplete = false;
	isPageLoading = false;
	reloadSubscription: any;

	// 2026 UI refresh: success/failure totals for the metric cards and an
	// endpoint scope for those counts. null means the total is unknown, which
	// the cards render as a dash; 0 means the window really held none.
	deliveryCounts: { success: number | null; failure: number | null } = { success: null, failure: null };
	isLoadingDeliveryCounts = false;
	private deliveryCountsFetchId = 0;
	summaryEndpoints: ENDPOINT[] = [];
	selectedSummaryEndpoint?: ENDPOINT;

	@ViewChild(EventDeliveriesComponent) eventDeliveriesTable?: EventDeliveriesComponent;

	constructor(private formBuilder: FormBuilder, private eventsService: EventsService, public privateService: PrivateService, public router: Router) {}

	async ngOnInit() {
		this.isloadingDashboardData = true;
		this.isPageLoading = true;
		await this.getProjectStats();

		if (this.isProjectConfigurationComplete) {
			await this.checkEventsOnFirstLoad();

			if (this.privateService.getProjectDetails?.type === 'incoming' && !this.hasEvents) {
				this.eventDelievryIntervalTime = setInterval(() => {
					this.getLatestEvent();
				}, 2000);
			}

			if (this.hasEvents) this.getEndpointsForSummary();

			this.isPageLoading = false;
			this.isloadingDashboardData = false;
		} else {
			this.isloadingDashboardData = false;
			this.isPageLoading = false;
		}
	}

	async getLatestSource() {
		try {
			const sources = await this.privateService.getSources();
			this.lastestSource = sources.data.content[sources.data.content.length - 1];
			return;
		} catch (error) {
			return error;
		}
	}

	async getLatestEvent() {
		try {
			const eventDeliveries = await this.eventsService.getEventDeliveries();
			this.lastestEventDeliveries = eventDeliveries.data.content;
			return;
		} catch (error) {
			this.isloadingDashboardData = false;
			this.isPageLoading = false;
			return error;
		}
	}

	async checkEventsOnFirstLoad() {
		if (this.hasEvents) {
			clearInterval(this.eventDelievryIntervalTime);
			this.isPageLoading = false;

			await this.fetchDashboardData();
			return;
		}

		if (this.privateService.getProjectDetails?.type === 'incoming' && this.isProjectConfigurationComplete) await this.getLatestSource();
	}

	setDateForFilter(requestDetails: { startDate: Date; endDate: Date; startTime?: string; endTime?: string }) {
		if (!requestDetails.endDate && !requestDetails.startDate) return { startDate: '', endDate: '' };
		const startDate = requestDetails.startDate ? `${format(requestDetails.startDate, 'yyyy-MM-dd')}${requestDetails?.startTime || 'T00:00:00'}` : '';
		const endDate = requestDetails.endDate ? `${format(requestDetails.endDate, 'yyyy-MM-dd')}${requestDetails?.endTime || 'T23:59:59'}` : '';
		return { startDate, endDate };
	}

	continueToDashboard() {
		this.fetchDashboardData();
		this.privateService.getProjectStat({ refresh: true });
		this.hasEvents = true;
		clearInterval(this.eventDelievryIntervalTime);
		this.getEndpointsForSummary();
	}

	// Formatted range for the date button, e.g. "12/02/2026 - 13/03/2026"
	get dateRangeLabel(): string {
		const { startDate, endDate } = this.statsDateRange.getRawValue();
		if (!startDate || !endDate) return 'Select date range';
		return `${format(new Date(startDate), 'dd/MM/yyyy')} - ${format(new Date(endDate), 'dd/MM/yyyy')}`;
	}

	get statsDateRangeForRequest(): { startDate: string; endDate: string } {
		const rawValue = this.statsDateRange.getRawValue();
		return typeof rawValue.startDate !== 'string' ? this.setDateForFilter(rawValue) : rawValue;
	}

	async fetchDashboardData() {
		const { startDate, endDate } = this.statsDateRangeForRequest;

		this.fetchDeliveryCounts();

		try {
			const dashboardResponse = await this.eventsService.dashboardSummary({ startDate, endDate, type: this.dashboardFrequency });
			this.dashboardData = dashboardResponse.data;
			this.initConvoyChart(dashboardResponse);

			this.isloadingDashboardData = false;
			return;
		} catch (error: any) {
			this.isloadingDashboardData = false;
			this.isPageLoading = false;
			return;
		}
	}

	// Successful/Failed totals, served from the daily rollup. null renders as a
	// dash, so a failed request cannot read as "no successful deliveries".
	async fetchDeliveryCounts() {
		const { startDate, endDate } = this.statsDateRangeForRequest;
		const endpointId = this.selectedSummaryEndpoint?.uid;
		const fetchId = ++this.deliveryCountsFetchId;

		this.isLoadingDeliveryCounts = true;
		const counts = await this.eventsService.getSummaryDeliveryCounts({ startDate, endDate, endpointId });

		// Only the latest request may write the cards or clear the flag, or an
		// earlier reply overwrites newer totals and hides the running spinner.
		if (fetchId !== this.deliveryCountsFetchId) return;
		this.deliveryCounts = counts;
		this.isLoadingDeliveryCounts = false;
	}

	async getEndpointsForSummary() {
		try {
			const response = await this.privateService.getEndpoints();
			this.summaryEndpoints = response.data.content || [];
		} catch (error) {}
	}

	selectSummaryEndpoint(endpoint?: ENDPOINT) {
		this.selectedSummaryEndpoint = endpoint;
		this.fetchDeliveryCounts();
	}

	getSelectedDateRange(dateRange?: { startDate: Date; endDate: Date }) {
		this.dateRangeValue = dateRange;
		this.statsDateRange.patchValue({
			startDate: dateRange?.startDate || new Date(new Date().setDate(new Date().getDate() - 30)),
			endDate: dateRange?.endDate || new Date()
		});
		this.fetchDashboardData();

		// The page-level date range also scopes the delivery table below.
		const { startDate, endDate } = this.statsDateRangeForRequest;
		this.eventDeliveriesTable?.applyDateFilter(dateRange ? { startDate, endDate } : undefined);
	}

	initConvoyChart(dashboardResponse: HTTP_RESPONSE) {
		let chartData: { label: string; data: any }[] = [];

		// Sort ascending so the oldest bucket sits on the left and today on the far right.
		const eventData = [...dashboardResponse.data.event_data].sort((a: any, b: any) => new Date(a.data.date).getTime() - new Date(b.data.date).getTime());
		const labelFormat = this.getDateLabelFormat();
		eventData.forEach((data: any) => {
			chartData.push({
				label: format(new Date(data.data.date), labelFormat),
				data: data.count || 0
			});
		});

		this.chartData = chartData;
	}

	get maxChartValue(): number {
		return Math.max(...(this.chartData || []).map(bucket => bucket.data), 1);
	}

	barHeight(count: number): number {
		// Keep a sliver visible for zero-count buckets, like the design.
		return Math.max((count / this.maxChartValue) * 100, 1.5);
	}

	getDateLabelFormat() {
		let labelsDateFormat = '';
		switch (this.dashboardFrequency) {
			case 'daily':
			case 'weekly':
				labelsDateFormat = 'dd-MM';
				break;
			case 'monthly':
				labelsDateFormat = 'MMM yy';
				break;
			case 'yearly':
				labelsDateFormat = 'yyyy';
				break;
			default:
				break;
		}

		return labelsDateFormat;
	}

	async getProjectStats() {
		try {
			const projectStats = await this.privateService.getProjectStat();
			this.isProjectConfigurationComplete = projectStats.data?.subscriptions_exist;
			this.hasEvents = projectStats.data?.events_exist;
			return;
		} catch (error) {
			return error;
		}
	}
}
