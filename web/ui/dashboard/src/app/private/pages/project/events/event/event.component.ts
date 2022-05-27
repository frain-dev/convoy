import { DatePipe } from '@angular/common';
import { Component, ElementRef, Input, OnInit, ViewChild } from '@angular/core';
import { FormBuilder, FormGroup } from '@angular/forms';
import { fromEvent, Observable } from 'rxjs';
import { debounceTime, distinctUntilChanged, map, startWith, switchMap } from 'rxjs/operators';
import { APP } from 'src/app/models/app.model';
import { EVENT, EVENT_DELIVERY } from 'src/app/models/event.model';
import { PAGINATION } from 'src/app/models/global.model';
import { TimeFilterComponent } from 'src/app/private/components/time-filter/time-filter.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { EventsService } from '../events.service';

@Component({
	selector: 'app-event',
	templateUrl: './event.component.html',
	styleUrls: ['../events.component.scss']
})
export class EventComponent implements OnInit {
	@Input() activeTab!: string;
	eventsFilterDateRange: FormGroup = this.formBuilder.group({
		startDate: [{ value: '', disabled: true }],
		endDate: [{ value: '', disabled: true }]
	});
	eventsTableHead: string[] = ['Event Type', 'App Name', 'Time Created', ''];
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	eventsSearchString!: string;
	eventsPage!: number;
	eventApp!: string;
	showEventFilterCalendar: boolean = false;
	showOverlay: boolean = false;
	showEventsAppsDropdown: boolean = false;
	isloadingEvents: boolean = false;
	selectedEventsDateOption: string = '';
	activeProjectId!: string;
	eventDeliveryFilteredByEventId!: string;
	filteredApps!: APP[];
	eventDetailsTabs = [
		{ id: 'data', label: 'Event' },
		{ id: 'response', label: 'Response' },
		{ id: 'request', label: 'Request' }
	];
	displayedEvents: {
		date: string;
		content: EVENT[];
	}[] = [
		{
			date: '28 Mar, 2022',
			content: [
				{
					app_metadata: {
						group_id: 'db78d6fe-b05e-476d-b908-cb6fff26a3ed',
						support_email: 'pelumi@mailinator.com',
						title: 'App B',
						uid: '73bd4f0e-e987-45b6-bf10-2d6da4ad3fe7'
					},
					created_at: '2022-03-28T16:22:51.972Z',
					data: '{test}',
					event_type: 'test.create',
					matched_endpoints: 1,
					provider_id: '73bd4f0e-e987-45b6-bf10-2d6da4ad3fe7',
					uid: 'a4495e71-1747-4869-842b-4bed9fb27f47',
					updated_at: '2022-03-28T16:22:51.972Z'
				}
			]
		}
	];
	events!: { pagination: PAGINATION; content: EVENT[] };
	eventDetailsActiveTab = 'data';
	eventsDetailsItem: any;
	sidebarEventDeliveries!: EVENT_DELIVERY[];
	eventsTimeFilterData: { startTime: string; endTime: string } = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
	eventDelsTimeFilterData: { startTime: string; endTime: string } = { startTime: 'T00:00:00', endTime: 'T23:59:59' };
	@ViewChild('eventsTimeFilter', { static: true }) eventsTimerFilter!: TimeFilterComponent;
	@ViewChild('eventsAppsFilter', { static: true }) eventsAppsFilter!: ElementRef;
	eventsAppsFilter$!: Observable<APP[]>;
	constructor(private formBuilder: FormBuilder, private eventsService: EventsService, private datePipe: DatePipe, private generalService: GeneralService) {}

	ngOnInit(): void {}
	ngAfterViewInit() {
		this.eventsAppsFilter$ = fromEvent<any>(this.eventsAppsFilter?.nativeElement, 'keyup').pipe(
			map(event => event.target.value),
			startWith(''),
			debounceTime(500),
			distinctUntilChanged(),
			switchMap(search => this.getAppsForFilter(search))
		);
	}
	getEvents(requestDetails?: any) {}

	clearEventFilters(filterType?: 'eventsDate' | 'eventsApp' | 'eventsSearch') {}

	async getAppsForFilter(search: string): Promise<APP[]> {
		return await (
			await this.eventsService.getApps({ activeProjectId: this.activeProjectId, pageNo: 1, searchString: search })
		).data.content;
	}

	updateAppFilter(appId: string, isChecked: any) {
		this.showOverlay = false;
		this.showEventsAppsDropdown = !this.showEventsAppsDropdown;
		isChecked.target.checked ? (this.eventApp = appId) : (this.eventApp = '');

		this.getEvents({ addToURL: true, fromFilter: true });
	}

	formatDate(date: Date) {
		return this.datePipe.transform(date, 'dd/MM/yyyy');
	}

	getSelectedDate(dateOption: string) {
		this.selectedEventsDateOption = dateOption;
		const { startDate, endDate } = this.generalService.getSelectedDate(dateOption);
		this.eventsFilterDateRange.patchValue({
			startDate: startDate,
			endDate: endDate
		});
		this.getEvents({ addToURL: true, fromFilter: true });
	}

	getCodeSnippetString() {
		if (!this.eventsDetailsItem?.data) return 'No event data was sent';
		return JSON.stringify(this.eventsDetailsItem?.data || this.eventsDetailsItem?.metadata?.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
	}

	async getEventDeliveriesForSidebar(eventId: string) {
		const response = await this.eventsService.getEventDeliveries({
			eventId,
			activeProjectId: this.activeProjectId,
			startDate: '',
			endDate: '',
			pageNo: 1,
			appId: '',
			statusQuery: ''
		});
		this.sidebarEventDeliveries = response.data.content;
	}
}
