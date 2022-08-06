import { Component, OnInit, ViewChild } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';
import { differenceInCalendarDays, differenceInCalendarMonths, differenceInCalendarWeeks, differenceInCalendarYears, format, getDayOfYear, getMonth, getWeek, getYear, sub } from 'date-fns';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import { EventsService } from './events.service';
import { EVENT, EVENT_DELIVERY } from 'src/app/models/event.model';
import { CHARTDATA, PAGINATION } from 'src/app/models/global.model';
import { PrivateService } from 'src/app/private/private.service';
import { ActivatedRoute, Router } from '@angular/router';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';

interface LABELS {
	date: string;
	index: number;
}

@Component({
	selector: 'app-events',
	templateUrl: './events.component.html',
	styleUrls: ['./events.component.scss']
})
export class EventsComponent implements OnInit {
	@ViewChild(DropdownComponent) dropdownComponent!: DropdownComponent;
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	tabs: ['events', 'event deliveries'] = ['events', 'event deliveries'];
	activeTab: 'events' | 'event deliveries' = 'events';
	showOverlay: boolean = false;
	isloadingDashboardData: boolean = false;
	showFilterDropdown: boolean = false;
	selectedDateOption: string = '';
	dashboardFrequency: 'daily' | 'weekly' | 'monthly' | 'yearly' = 'daily';
	filterOptions: ['daily', 'weekly', 'monthly', 'yearly'] = ['daily', 'weekly', 'monthly', 'yearly'];
	dashboardData = { apps: 0, events_sent: 0 };
	eventDeliveryFilteredByEventId!: string;
	events!: { pagination: PAGINATION; content: EVENT[] };
	eventDeliveries!: { pagination: PAGINATION; content: EVENT_DELIVERY[] };
	statsDateRange: FormGroup = this.formBuilder.group({
		startDate: [{ value: new Date(new Date().setDate(new Date().getDate() - 30)), disabled: true }],
		endDate: [{ value: new Date(), disabled: true }]
	});
	chartData!: CHARTDATA[];

	constructor(private formBuilder: FormBuilder, private eventsService: EventsService, public privateService: PrivateService, private route: ActivatedRoute, private router: Router) {
		this.fetchDashboardData();
	}

	async ngOnInit() {
		this.toggleActiveTab(this.route.snapshot.queryParams?.activeTab ?? 'events');
	}

	closeFilterOptions() {
		this.dropdownComponent.show = false;
	}

	addTabToUrl() {
		const currentURLfilters = this.route.snapshot.queryParams;
		const queryParams: any = {};

		queryParams.activeTab = this.activeTab;
		this.router.navigate([], { queryParams: Object.assign({}, currentURLfilters, queryParams) });
	}

	toggleActiveTab(tab: 'events' | 'event deliveries') {
		this.activeTab = tab;
		this.addTabToUrl();
	}

	async fetchDashboardData() {
		try {
			this.isloadingDashboardData = true;
			const { startDate, endDate } = this.setDateForFilter(this.statsDateRange.value);

			const dashboardResponse = await this.eventsService.dashboardSummary({ startDate: startDate || '', endDate: endDate || '', frequency: this.dashboardFrequency });
			this.dashboardData = dashboardResponse.data;
			const chatLabels = this.getDateRange();
			this.initConvoyChart(dashboardResponse, chatLabels);

			this.isloadingDashboardData = false;
		} catch (error: any) {
			this.isloadingDashboardData = false;
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

	getEventDeliveries(eventId: string) {
		this.eventDeliveryFilteredByEventId = eventId;
		this.toggleActiveTab('event deliveries');
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
		// const difference = chartData.length % 6 > 0 ? (chartData.length + 5) / 6 : chartData.length / 6;
		// const periodDiff = [this.chartData[0], this.chartData[difference], this.chartData[difference + 6]];
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

	getDateRange() {
		const { startDate, endDate } = this.setDateForFilter(this.statsDateRange.value);
		return this.dateRange(startDate, endDate);
	}
}
