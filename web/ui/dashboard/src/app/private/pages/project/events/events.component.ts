import { Component, OnDestroy, OnInit } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';
import { differenceInCalendarDays, differenceInCalendarMonths, differenceInCalendarWeeks, differenceInCalendarYears, format, getDayOfYear, getMonth, getWeek, getYear, sub } from 'date-fns';
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
	lastestSource!: SOURCE;
	lastestEventDeliveries: EVENT_DELIVERY[] = [];
	eventDelTableHead: string[] = ['Status', 'Event Type', 'Event Time', 'Next Attempt'];
	eventDelievryIntervalTime: any;

	constructor(private formBuilder: FormBuilder, private eventsService: EventsService, public privateService: PrivateService, public router: Router) {}

	async ngOnInit() {
		this.isloadingDashboardData = true;
		await Promise.all([this.fetchDashboardData(), this.getLatestSource(), this.getLatestEvent()]);
		this.checkEventsOnFirstLoad();
		this.isloadingDashboardData = false;

		if (this.privateService.activeProjectDetails?.type === 'incoming') {
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
			const eventDeliveries = await this.eventsService.getEventDeliveries({ pageNo: 1 });
			this.lastestEventDeliveries = eventDeliveries.data.content;
			this.privateService.activeProjectDetails?.type === 'outgoing' && this.lastestEventDeliveries.length > 0;
			return;
		} catch (error) {
			return error;
		}
	}

	async checkEventsOnFirstLoad() {
		this.hasEvents = this.lastestEventDeliveries.length === 0 ? false : true;
		clearInterval(this.eventDelievryIntervalTime);
	}

	async fetchDashboardData() {
		try {
			const { startDate, endDate } = this.setDateForFilter(this.statsDateRange.value);

			const dashboardResponse = await this.eventsService.dashboardSummary({ startDate: startDate || '', endDate: endDate || '', frequency: this.dashboardFrequency });
			this.dashboardData = dashboardResponse.data;
			const chatLabels = this.getDateRange();
			this.initConvoyChart(dashboardResponse, chatLabels);

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

	initConvoyChart(dashboardResponse: HTTP_RESPONSE, chatLabels: LABELS[]) {
		let chartData: { label: string; data: any }[] = [];

		const eventData = dashboardResponse.data.event_data;

		chatLabels.forEach(label => {
			chartData.push({
				label: label.date,
				data: eventData.find((data: { data: { index: number } }) => data.data.index === label.index)?.count || 0
			});
		});
		this.chartData = chartData;
	}

	dateRange(startDate: string, endDate: string): { date: string; index: number }[] {
		let labelsDateFormat = '';
		let periodDifference;
		let currentDate = new Date(startDate);
		let currentEndDate = new Date(endDate);
		let currentStartDate = currentDate;

		switch (this.dashboardFrequency) {
			case 'daily':
				labelsDateFormat = 'do, MMM';
				periodDifference = differenceInCalendarDays(new Date(endDate), new Date(startDate)) + 1;
				periodDifference && periodDifference < 31 ? (currentStartDate = sub(currentEndDate, { days: 30 })) : (currentStartDate = currentDate);
				break;
			case 'weekly':
				labelsDateFormat = 'yyyy-MM';
				periodDifference = differenceInCalendarWeeks(new Date(endDate), new Date(startDate)) + 1;
				periodDifference && periodDifference < 31 ? (currentStartDate = sub(currentEndDate, { weeks: 30 })) : (currentStartDate = currentDate);
				break;
			case 'monthly':
				labelsDateFormat = 'MMM';
				periodDifference = differenceInCalendarMonths(new Date(endDate), new Date(startDate)) + 1;
				periodDifference && periodDifference < 31 ? (currentStartDate = sub(currentEndDate, { months: 30 })) : (currentStartDate = currentDate);
				break;
			case 'yearly':
				labelsDateFormat = 'yyyy';
				periodDifference = differenceInCalendarYears(new Date(endDate), new Date(startDate)) + 1;
				periodDifference && periodDifference < 31 ? (currentStartDate = sub(currentEndDate, { years: 30 })) : (currentStartDate = currentDate);
				break;
			default:
				break;
		}

		for (var dateArray: any = []; currentStartDate <= currentEndDate; currentStartDate.setDate(currentStartDate.getDate() + 1)) {
			switch (this.dashboardFrequency) {
				case 'daily':
					dateArray.push({
						index: getDayOfYear(new Date(currentStartDate)),
						date: format(new Date(currentStartDate), labelsDateFormat)
					});
					break;
				case 'weekly':
					dateArray.push({
						index: getWeek(new Date(currentStartDate)),
						date: format(new Date(currentStartDate), labelsDateFormat)
					});
					break;
				case 'monthly':
					dateArray.push({
						index: getMonth(new Date(currentStartDate)) + 1,
						date: format(new Date(currentStartDate), labelsDateFormat)
					});
					break;
				case 'yearly':
					dateArray.push({
						index: getYear(new Date(currentStartDate)),
						date: format(new Date(currentStartDate), labelsDateFormat)
					});
					break;
				default:
					break;
			}

			dateArray = [...new Map(dateArray.map((item: any) => [item['index'], item])).values()];
		}
		return dateArray;
	}

	get isProjectConfigurationComplete() {
		const configurationComplete = localStorage.getItem('isActiveProjectConfigurationComplete');
		return configurationComplete ? JSON.parse(configurationComplete) : false;
	}

	getDateRange() {
		const { startDate, endDate } = this.setDateForFilter(this.statsDateRange.value);
		return this.dateRange(startDate, endDate);
	}
}
