import { Component, OnDestroy, OnInit } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';
import { format } from 'date-fns';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { EventsService } from './events.service';
import { EVENT_DELIVERY } from 'src/app/models/event.model';
import { CHARTDATA, PAGINATION } from 'src/app/models/global.model';
import { PrivateService } from 'src/app/private/private.service';
import { Router } from '@angular/router';
import { SOURCE } from 'src/app/models/group.model';

interface LABELS {
	date: string;
	index: number;
}

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

	constructor(private formBuilder: FormBuilder, private eventsService: EventsService, public privateService: PrivateService, public router: Router) {}

	async ngOnInit() {
		this.isloadingDashboardData = true;
		await this.getLatestEvent();
		this.checkEventsOnFirstLoad();
		this.isloadingDashboardData = false;

		if (this.privateService.activeProjectDetails?.type === 'incoming' && !this.lastestEventDeliveries.length) {
			this.eventDelievryIntervalTime = setInterval(() => {
				this.getLatestEvent();
			}, 2000);
		}
	}

	ngOnDestroy(): void {
		clearInterval(this.eventDelievryIntervalTime);
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
			return error;
		}
	}

	async checkEventsOnFirstLoad() {
		this.hasEvents = this.lastestEventDeliveries.length === 0 ? false : true;

		if (this.hasEvents) {
			clearInterval(this.eventDelievryIntervalTime);
			this.fetchDashboardData();
			return;
		}

		if (this.privateService.activeProjectDetails?.type === 'incoming' && this.isProjectConfigurationComplete) this.getLatestSource();
	}

	continueToDashboard() {
		this.fetchDashboardData();
		this.hasEvents = true;
		clearInterval(this.eventDelievryIntervalTime);
	}

	async fetchDashboardData() {
		try {
			const { startDate, endDate } = this.setDateForFilter(this.statsDateRange.value);

			const dashboardResponse = await this.eventsService.dashboardSummary({ startDate: startDate || '', endDate: endDate || '', type: this.dashboardFrequency });
			this.dashboardData = dashboardResponse.data;
			this.initConvoyChart(dashboardResponse);

			return;
		} catch (error: any) {
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

	get isProjectConfigurationComplete() {
		const configurationComplete = localStorage.getItem('isActiveProjectConfigurationComplete');
		return configurationComplete ? JSON.parse(configurationComplete) : false;
	}
}
