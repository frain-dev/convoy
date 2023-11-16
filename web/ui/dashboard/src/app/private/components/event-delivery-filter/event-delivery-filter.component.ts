import { Component, ElementRef, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { CommonModule, Location } from '@angular/common';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { DatePickerComponent } from 'src/app/components/date-picker/date-picker.component';
import { EndpointFilterComponent } from '../endpoints-filter/endpoints-filter.component';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { FILTER_QUERY_PARAM } from 'src/app/models/event.model';
import { ActivatedRoute } from '@angular/router';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { ProjectService } from '../../pages/project/project.service';
import { SOURCE } from 'src/app/models/source.model';
import { PrivateService } from '../../private.service';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { FormsModule } from '@angular/forms';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'convoy-event-delivery-filter',
	standalone: true,
	imports: [CommonModule, ButtonComponent, DatePickerComponent, EndpointFilterComponent, DropdownComponent, DropdownOptionDirective, ListItemComponent, FormsModule],
	templateUrl: './event-delivery-filter.component.html',
	styleUrls: ['./event-delivery-filter.component.scss']
})
export class EventDeliveryFilterComponent implements OnInit {
	@ViewChild('datePicker', { static: true }) datePicker!: DatePickerComponent;
	@ViewChild('eventTypeFilter', { static: false }) eventTypeFilter?: any;

	@Input('type') type: 'deliveries' | 'logs' = 'deliveries';

	@Output('sortEvents') sort = new EventEmitter<any>();
	@Output('filter') filter = new EventEmitter<any>();
	@Output('tail') tail = new EventEmitter<any>();
	@Output('batchRetry') batchRetry = new EventEmitter<any>();

	sortOrder: 'asc' | 'desc' | string = 'desc';

	eventDeliveryStatuses = ['Success', 'Failure', 'Retry', 'Scheduled', 'Processing', 'Discarded'];
	eventDeliveryFilteredByStatus: string[] = [];

	eventDeliveriesSource?: string;
	eventDeliveriesSourceData?: SOURCE;
	filterSources?: SOURCE[];

	eventDeliveriesEndpoint?: string;
	eventDeliveriesEndpointData?: ENDPOINT;

	eventDelEventType?: string;
	eventsTypeSearchString!: string;

	eventsSearchString!: string;

	portalToken = this.route.snapshot.queryParams?.token;

	queryParams: FILTER_QUERY_PARAM = {};
	enableTailMode = false;
	filterOptions = [
		{ name: 'Date', id: 'date', show: false },
		{ name: 'Status', id: 'status', show: false },
		{ name: 'Source', id: 'source', show: false },
		{ name: 'Endpoint', id: 'endpoint', show: false },
		{ name: 'Event type', id: 'eventType', show: false }
	];
	constructor(private route: ActivatedRoute, private _location: Location, public projectService: ProjectService, private privateService: PrivateService, private generalService: GeneralService) {}

	async ngOnInit() {
		const data = this.getFiltersFromURL();
		if (this.checkIfTailModeIsEnabled()) this.tail.emit({ data: this.queryParams, tailModeConfig: this.checkIfTailModeIsEnabled() });
		this.filter.emit(data);

		if (this.type === 'logs') this.projectService.activeProjectDetails?.type == 'outgoing' ? this.filterOptions.splice(1, 4) : this.filterOptions.splice(1, 4, { name: 'Source', id: 'source', show: false });
		else this.projectService.activeProjectDetails?.type == 'incoming' ? this.filterOptions.splice(3, 2) : this.filterOptions.splice(2, 1);

		if (this.eventDeliveriesSource) this.eventDeliveriesSourceData = await this.getSelectedSourceData();

		if (this.eventDeliveriesEndpoint) this.eventDeliveriesEndpointData = await this.getSelectedEndpointData();

		if (!this.portalToken || this.projectService.activeProjectDetails?.type == 'incoming') this.getSourcesForFilter();
	}

	getFiltersFromURL() {
		this.queryParams = { ...this.queryParams, ...this.route.snapshot.queryParams };

		// set filter status if any exists in URL
		this.eventDeliveryFilteredByStatus = this.queryParams.status ? JSON.parse(this.queryParams.status) : [];

		this.eventDeliveriesSource = this.queryParams?.sourceId;
		this.eventDeliveriesEndpoint = this.queryParams?.endpointId;

		this.eventDelEventType = this.queryParams?.eventType;

		this.sortOrder = this.queryParams?.sort || 'desc';

		this.eventsSearchString = this.queryParams?.query || '';

		return this.queryParams;
	}

	clearFilters(filterType?: 'startDate' | 'endDate' | 'eventId' | 'endpointId' | 'status' | 'sourceId' | 'next_page_cursor' | 'prev_page_cursor' | 'direction' | 'eventType') {
		if (filterType && this.queryParams) {
			// if filter to clear start date or end date, it means clear date filter. :)
			if (filterType === 'startDate' || filterType === 'endDate') {
				delete this.queryParams['startDate'];
				delete this.queryParams['endDate'];
				this.datePicker?.clearDate();
			} else if (filterType === 'sourceId') {
				this.eventDeliveriesSource = '';
				delete this.queryParams['sourceId'];
			} else if (filterType === 'endpointId') {
				this.eventDeliveriesEndpoint = '';
				delete this.queryParams['endpointId'];
			} else if (filterType === 'eventType') {
				this.eventDelEventType = '';
				this.eventsTypeSearchString = '';
				delete this.queryParams['eventType'];
			} else if (filterType === 'eventId') {
				delete this.queryParams['eventId'];
				delete this.queryParams['idempotencyKey'];
			} else delete this.queryParams[filterType];

			const cleanedQuery: any = Object.fromEntries(Object.entries(this.queryParams).filter(([_, q]) => q !== '' && q !== undefined && q !== null));
			const queryParams = new URLSearchParams(cleanedQuery).toString();
			this._location.go(`${location.pathname}?${this.portalToken ? `token=${this.portalToken}` : ''}${queryParams}`);
		} else {
			this.eventDelEventType = '';
			this.eventsTypeSearchString = '';
			this.eventDeliveriesSource = '';
			this.eventDeliveriesEndpoint = '';
			this.eventDeliveryFilteredByStatus = [];
			this.datePicker?.clearDate();
			const sortParam = this.queryParams.sort;

			this.queryParams = {};
			this._location.go(`${location.pathname}${this.portalToken ? `?token=${this.portalToken}` : ''}`);
		}

		this.checkIfTailModeIsEnabled() ? this.toggleTailMode(false, 'on') : this.toggleTailMode(false, 'off');
		this.filter.emit();
	}

	getSelectedDateRange(dateRange: { startDate: string; endDate: string }) {
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, ...dateRange });
		this.checkIfTailModeIsEnabled() ? this.toggleTailMode(false, 'on') : this.toggleTailMode(false, 'off');
		this.filter.emit(this.queryParams);
	}

	selectStatusFilter(status: string) {
		if (!this.eventDeliveryFilteredByStatus?.includes(status)) this.eventDeliveryFilteredByStatus.push(status);
	}

	removeStatusFilter(status: string) {
		this.eventDeliveryFilteredByStatus = this.eventDeliveryFilteredByStatus.filter(e => e !== status);
		this.getSelectedStatusFilter();
	}

	getSelectedStatusFilter() {
		const eventDelsStatus = this.eventDeliveryFilteredByStatus.length > 0 ? JSON.stringify(this.eventDeliveryFilteredByStatus) : '';
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, status: eventDelsStatus });
		this.checkIfTailModeIsEnabled() ? this.toggleTailMode(false, 'on') : this.toggleTailMode(false, 'off');
		this.filter.emit(this.queryParams);
		this.toggleFilter('status', false);
	}

	updateSourceFilter(source: SOURCE) {
		this.eventDeliveriesSource = source.uid;
		this.eventDeliveriesSourceData = source;
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, sourceId: this.eventDeliveriesSource });
		this.checkIfTailModeIsEnabled() ? this.toggleTailMode(false, 'on') : this.toggleTailMode(false, 'off');
		this.filter.emit(this.queryParams);
	}

	updateEndpointFilter(endpoint: ENDPOINT) {
		this.eventDeliveriesEndpoint = endpoint.uid;
		this.eventDeliveriesEndpointData = endpoint;
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, endpointId: this.eventDeliveriesEndpoint });
		this.checkIfTailModeIsEnabled() ? this.toggleTailMode(false, 'on') : this.toggleTailMode(false, 'off');
		this.filter.emit(this.queryParams);
	}

	searchEvents() {
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, query: this.eventsSearchString });
		this.checkIfTailModeIsEnabled() ? this.toggleTailMode(false, 'on') : this.toggleTailMode(false, 'off');
		this.filter.emit(this.queryParams);
	}

	setEventType() {
		this.eventDelEventType = this.eventsTypeSearchString;
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, eventType: this.eventsTypeSearchString });
		this.checkIfTailModeIsEnabled() ? this.toggleTailMode(false, 'on') : this.toggleTailMode(false, 'off');
		this.filter.emit(this.queryParams);
		this.toggleFilter('eventType', false);
	}

	toggleSortOrder() {
		this.sortOrder === 'asc' ? (this.sortOrder = 'desc') : (this.sortOrder = 'asc');
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, sort: this.sortOrder });
		// this.type === 'logs' ? localStorage.setItem('EVENTS_LOGS_SORT_ORDER', this.sortOrder) : localStorage.setItem('EVENTS_SORT_ORDER', this.sortOrder);
		this.checkIfTailModeIsEnabled() ? this.toggleTailMode(false, 'on') : this.toggleTailMode(false, 'off');
		this.filter.emit(this.queryParams);
	}

	// getSortOrder() {
	// 	const sortOrderConfig = this.type === 'logs' ? localStorage.getItem('EVENTS_LOGS_SORT_ORDER') : localStorage.getItem('EVENTS_SORT_ORDER');
	// 	this.sortOrder = sortOrderConfig || 'desc';
	// }

	toggleTailMode(e?: any, status?: 'on' | 'off') {
		let tailModeConfig: boolean;
		if (status) tailModeConfig = status === 'on';
		else tailModeConfig = e.target.checked;

		this.enableTailMode = tailModeConfig;
		this.type === 'logs' ? localStorage.setItem('EVENT_LOGS_TAIL_MODE', JSON.stringify(tailModeConfig)) : localStorage.setItem('EVENTS_TAIL_MODE', JSON.stringify(tailModeConfig));

		this.tail.emit({ data: this.queryParams, tailModeConfig });
	}

	checkIfTailModeIsEnabled() {
		const tailModeConfig = this.type === 'logs' ? localStorage.getItem('EVENT_LOGS_TAIL_MODE') : localStorage.getItem('EVENTS_TAIL_MODE');
		this.enableTailMode = tailModeConfig ? JSON.parse(tailModeConfig) : false;

		return this.enableTailMode;
	}

	toggleFilter(filterValue: string, show: boolean) {
		this.filterOptions.forEach(filter => {
			if (filter.id === filterValue) filter.show = show;
		});
	}

	showFilter(filterValue: string): boolean {
		return this.filterOptions.find(filter => filter.id === filterValue)?.show || false;
	}

	isAnyFilterSelected(): Boolean {
		return (
			(this.queryParams &&
				(Object.keys(this.queryParams).includes('sort') || !Object.keys(this.queryParams).includes('sort')) &&
				((Object.keys(this.queryParams).length > 0 && !Object.keys(this.queryParams).includes('sort')) || (Object.keys(this.queryParams).length > 1 && Object.keys(this.queryParams).includes('sort')))) ||
			false
		);
	}

	async getSelectedEndpointData(): Promise<ENDPOINT> {
		return await (await this.privateService.getEndpoints()).data.content.find((item: ENDPOINT) => item.uid === this.eventDeliveriesEndpoint);
	}

	async getSelectedSourceData(): Promise<SOURCE> {
		return await (await this.privateService.getSources()).data.content.find((item: SOURCE) => item.uid === this.eventDeliveriesSource);
	}

	async getSourcesForFilter() {
		try {
			const sourcesResponse = (await this.privateService.getSources()).data.content;
			this.filterSources = sourcesResponse;
		} catch (error) {}
	}

	showBatchRetry() {
		this.batchRetry.emit(this.queryParams);
	}
}
