import { DatePipe } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';
import { format } from 'date-fns';
import { HTTP_RESPONSE } from 'src/app/models/http.model';
import Chart from 'chart.js/auto';
import { EventsService } from './events.service';
import { EVENT, EVENT_DELIVERY } from 'src/app/models/event.model';
import { PAGINATION } from 'src/app/models/global.model';
import { PrivateService } from 'src/app/private/private.service';
import { ActivatedRoute, Router } from '@angular/router';

@Component({
	selector: 'app-events',
	templateUrl: './events.component.html',
	styleUrls: ['./events.component.scss']
})
export class EventsComponent implements OnInit {
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	tabs: ['events', 'event deliveries'] = ['events', 'event deliveries'];
	activeTab: 'events' | 'event deliveries' = 'events';
	showOverlay: boolean = false;
	isloadingDashboardData: boolean = false;
	showFilterCalendar: boolean = false;
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

	constructor(
		private formBuilder: FormBuilder,
		private datePipe: DatePipe,
		private eventsService: EventsService,
		public privateService: PrivateService,
		private route: ActivatedRoute,
		private router: Router
	) {}

	async ngOnInit() {
		this.toggleActiveTab(this.route.snapshot.queryParams?.activeTab ?? 'events');
		await this.fetchDashboardData();
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

	formatDate(date: Date) {
		return this.datePipe.transform(date, 'dd/MM/yyyy');
	}

	async fetchDashboardData() {
		try {
			this.isloadingDashboardData = true;
			const { startDate, endDate } = this.setDateForFilter(this.statsDateRange.value);

			const dashboardResponse = await this.eventsService.dashboardSummary({ startDate: startDate || '', endDate: endDate || '', frequency: this.dashboardFrequency });
			this.dashboardData = dashboardResponse.data;
			this.initChart(dashboardResponse);

			this.isloadingDashboardData = false;
		} catch (error: any) {
			this.isloadingDashboardData = false;
		}
	}

	getSelectedDateRange(dateRange: { startDate: Date; endDate: Date }) {
		this.statsDateRange.patchValue({
			startDate: dateRange.startDate,
			endDate: dateRange.endDate
		});
		this.fetchDashboardData();
	}

	setDateForFilter(requestDetails: { startDate: Date; endDate: Date; startTime?: string; endTime?: string }) {
		if (!requestDetails.endDate && !requestDetails.startDate) return { startDate: '', endDate: '' };
		const startDate = requestDetails.startDate ? `${format(requestDetails.startDate, 'yyyy-MM-dd')}${requestDetails?.startTime || 'T00:00:00'}` : '';
		const endDate = requestDetails.endDate ? `${format(requestDetails.endDate, 'yyyy-MM-dd')}${requestDetails?.endTime || 'T23:59:59'}` : '';
		return { startDate, endDate };
	}

	initChart(dashboardResponse: HTTP_RESPONSE) {
		let labelsDateFormat = '';
		if (this.dashboardFrequency === 'daily') labelsDateFormat = 'do, MMM';
		else if (this.dashboardFrequency === 'monthly') labelsDateFormat = 'MMM';
		else if (this.dashboardFrequency === 'yearly') labelsDateFormat = 'yyyy';

		const chartData = dashboardResponse.data.event_data;
		const labels = [...chartData.map((label: { data: { date: any } }) => label.data.date)].map(date => {
			return this.dashboardFrequency === 'weekly' ? date : format(new Date(date), labelsDateFormat);
		});
		labels.unshift('0');
		const dataSet = [0, ...chartData.map((label: { count: any }) => label.count)];
		const data = {
			labels,
			datasets: [
				{
					data: dataSet,
					fill: false,
					borderColor: '#477DB3',
					tension: 0.5,
					yAxisID: 'yAxis',
					xAxisID: 'xAxis'
				}
			]
		};

		const options = {
			plugins: {
				legend: {
					display: false
				}
			},
			scales: {
				xAxis: {
					display: true,
					grid: {
						display: false
					}
				}
			}
		};

		if (!Chart.getChart('dahboard_events_chart') || !Chart.getChart('dahboard_events_chart')?.canvas) {
			new Chart('dahboard_events_chart', { type: 'line', data, options });
		} else {
			const currentChart = Chart.getChart('dahboard_events_chart');
			if (currentChart) {
				currentChart.data.labels = labels;
				currentChart.data.datasets[0].data = dataSet;
				currentChart.update();
			}
		}
	}

	getEventDeliveries(eventId: string) {
		this.eventDeliveryFilteredByEventId = eventId;
		this.toggleActiveTab('event deliveries');
	}
}
