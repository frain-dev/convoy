import { Component, OnDestroy, OnInit } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';
import { format } from 'date-fns';
import { HTTP_RESPONSE } from 'src/app/models/global.model';
import { EventsService } from './events.service';
import { EVENT_DELIVERY } from 'src/app/models/event.model';
import { CHARTDATA, PAGINATION } from 'src/app/models/global.model';
import { PrivateService } from 'src/app/private/private.service';
import { NavigationEnd, Router } from '@angular/router';
import { SOURCE } from 'src/app/models/source.model';

@Component({
	selector: 'app-events',
	templateUrl: './events.component.html',
	styleUrls: ['./events.component.scss']
})
export class EventsComponent implements OnInit, OnDestroy {
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	isloadingDashboardData: boolean = false;
	showFilterDropdown: boolean = false;
	selectedDateOption: string = '';
	dashboardFrequency: 'daily' | 'weekly' | 'monthly' | 'yearly' = 'daily';
	filterOptions: ['daily', 'weekly', 'monthly', 'yearly'] = ['daily', 'weekly', 'monthly', 'yearly'];
	dashboardData = { apps: 0, events_sent: 0 };
	eventDeliveries!: { pagination: PAGINATION; content: EVENT_DELIVERY[] };
	statsDateRange: FormGroup = this.formBuilder.group({
		startDate: [{ value: new Date(new Date().setDate(new Date().getDate() - 30)), disabled: true }],
		endDate: [{ value: new Date(), disabled: true }]
	});
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

	constructor(private formBuilder: FormBuilder, private eventsService: EventsService, public privateService: PrivateService, public router: Router) {
		// for reloading this component when the same route is called again
		this.router.routeReuseStrategy.shouldReuseRoute = function () {
			return false;
		};

		this.reloadSubscription = this.router.events.subscribe(event => {
			if (event instanceof NavigationEnd) {
				this.router.navigated = false;
			}
		});
	}

	async ngOnInit() {
		this.isloadingDashboardData = true;
		this.isPageLoading = true;
		await this.getProjectStats();

		if (this.isProjectConfigurationComplete) {
			await this.checkEventsOnFirstLoad();

			if (this.privateService.activeProjectDetails?.type === 'incoming' && !this.hasEvents) {
				this.eventDelievryIntervalTime = setInterval(() => {
					this.getLatestEvent();
				}, 2000);
			}

			this.isPageLoading = false;
			this.isloadingDashboardData = false;
		} else {
			this.isloadingDashboardData = false;
			this.isPageLoading = false;
		}
	}

	ngOnDestroy(): void {
		clearInterval(this.eventDelievryIntervalTime);
		this.reloadSubscription?.unsubscribe();
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

		if (this.privateService.activeProjectDetails?.type === 'incoming' && this.isProjectConfigurationComplete) await this.getLatestSource();
	}

	continueToDashboard() {
		this.fetchDashboardData();
		this.privateService.getProjectStat({ refresh: true });
		this.hasEvents = true;
		clearInterval(this.eventDelievryIntervalTime);
	}

	async fetchDashboardData() {
		try {
			const { startDate, endDate } = this.setDateForFilter(this.statsDateRange.value);

			const dashboardResponse = await this.eventsService.dashboardSummary({ startDate: startDate || '', endDate: endDate || '', type: this.dashboardFrequency });
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

	getSelectedDateRange(dateRange?: { startDate: Date; endDate: Date }) {
		this.statsDateRange.patchValue({
			startDate: dateRange?.startDate || new Date(new Date().setDate(new Date().getDate() - 30)),
			endDate: dateRange?.endDate || new Date()
		});
		this.fetchDashboardData();
	}

	setDateForFilter(requestDetails: { startDate: Date; endDate: Date; startTime?: string; endTime?: string }) {
		if (!requestDetails.endDate && !requestDetails.startDate) return { startDate: '', endDate: '' };
		const startDate = requestDetails.startDate ? `${format(requestDetails.startDate, 'yyyy-MM-dd')}${requestDetails?.startTime || 'T00:00:00'}` : '';
		const endDate = requestDetails.endDate ? `${format(requestDetails.endDate, 'yyyy-MM-dd')}${requestDetails?.endTime || 'T23:59:59'}` : '';
		return { startDate, endDate };
	}

	initConvoyChart(dashboardResponse: HTTP_RESPONSE) {
		let chartData: { label: string; data: any }[] = [];

		const eventData = dashboardResponse.data.event_data.reverse();
		const labelFormat = this.getDateLabelFormat();
		eventData.forEach((data: any) => {
			chartData.push({
				label: format(new Date(data.data.date), labelFormat),
				data: data.count || 0
			});
		});

		this.chartData = chartData;
	}

	getDateLabelFormat() {
		let labelsDateFormat = '';
		switch (this.dashboardFrequency) {
			case 'daily':
				labelsDateFormat = 'do, MMM, yyyy';
				break;
			case 'weekly':
				labelsDateFormat = 'yyyy-MM';
				break;
			case 'monthly':
				labelsDateFormat = 'MMM, yyyy';
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
			this.isProjectConfigurationComplete = projectStats.data?.total_subscriptions > 0;
			this.hasEvents = projectStats.data?.messages_sent > 0;
			return;
		} catch (error) {
			return error;
		}
	}
}
