import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { TableComponent, TableCellComponent, TableRowComponent, TableHeadCellComponent, TableHeadComponent } from 'src/app/components/table/table.component';
import { EventLogsService } from './event-logs.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { SOURCE } from 'src/app/models/source.model';
import { EVENT, EVENT_DELIVERY, FILTER_QUERY_PARAM } from 'src/app/models/event.model';
import { DatePickerComponent } from 'src/app/components/date-picker/date-picker.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { PrismModule } from 'src/app/private/components/prism/prism.module';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { FormsModule } from '@angular/forms';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { EventsService } from '../events/events.service';
import { PaginationComponent } from 'src/app/private/components/pagination/pagination.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { EventDeliveryFilterComponent } from 'src/app/private/components/event-delivery-filter/event-delivery-filter.component';

@Component({
	selector: 'convoy-event-logs',
	standalone: true,
	imports: [
		CommonModule,
		RouterModule,
		FormsModule,
		StatusColorModule,
		PrismModule,
		LoaderModule,
		CardComponent,
		ButtonComponent,
		EmptyStateComponent,
		TagComponent,
		TableLoaderModule,
		TableComponent,
		TableHeadComponent,
		TableRowComponent,
		TableHeadCellComponent,
		TableCellComponent,
		DatePickerComponent,
		DropdownComponent,
		PaginationComponent,
		CopyButtonComponent,
		ListItemComponent,
		DropdownOptionDirective,
		DialogDirective,
		EventDeliveryFilterComponent
	],
	templateUrl: './event-logs.component.html',
	styleUrls: ['./event-logs.component.scss']
})
export class EventLogsComponent implements OnInit {
	@ViewChild('batchDialog', { static: true }) batchDialog!: ElementRef<HTMLDialogElement>;
	eventsDateFilterFromURL: { startDate: string; endDate: string } = { startDate: '', endDate: '' };
	eventLogsTableHead: string[] = ['Event ID', 'Source', 'Time', ''];
	dateOptions = ['Last Year', 'Last Month', 'Last Week', 'Yesterday'];
	eventSource?: string;
	isloadingEvents: boolean = false;
	eventDetailsTabs = [
		{ id: 'data', label: 'Event' },
		{ id: 'response', label: 'Response' },
		{ id: 'request', label: 'Request' }
	];
	displayedEvents: { date: string; content: EVENT[] }[] = [];
	events?: { pagination: PAGINATION; content: EVENT[] };
	duplicateEvents!: EVENT[];
	eventDetailsActiveTab = 'data';
	eventsDetailsItem: any;
	sidebarEventDeliveries: EVENT_DELIVERY[] = [];
	@ViewChild('datePicker', { static: true }) datePicker!: DatePickerComponent;
	portalToken = this.route.snapshot.params?.token;
	filterSources: SOURCE[] = [];
	isLoadingSidebarDeliveries = true;
	fetchingCount = false;
	isRetrying = false;
	isFetchingDuplicateEvents = false;
	batchRetryCount: any;
	getEventsInterval: any;
	queryParams: FILTER_QUERY_PARAM = {};
	enableTailMode = false;
	sortOrder: 'asc' | 'desc' = 'desc';

	constructor(private eventsLogService: EventLogsService, public generalService: GeneralService, public route: ActivatedRoute, private router: Router, public privateService: PrivateService, private eventsService: EventsService) {}

	ngOnInit() {}

	ngOnDestroy() {
		clearInterval(this.getEventsInterval);
	}

	fetchEventLogs(requestDetails: FILTER_QUERY_PARAM) {
		const data = requestDetails;
		this.queryParams = data;
		this.getEventLogs({ ...data, showLoader: true });
	}

	handleTailing(tailDetails: { data: FILTER_QUERY_PARAM; tailModeConfig: boolean }) {
		this.queryParams = tailDetails.data;

		clearInterval(this.getEventsInterval);
		if (tailDetails.tailModeConfig) this.getEventsAtInterval(tailDetails.data);
	}

	getEventsAtInterval(data: FILTER_QUERY_PARAM) {
		this.getEventsInterval = setInterval(() => {
			this.getEventLogs(data);
		}, 5000);
	}

	paginateEvents(event: CURSOR) {
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, ...event });
		this.handleTailing({ data: this.queryParams, tailModeConfig: this.checkIfTailModeIsEnabled() });
		this.getEventLogs({ ...this.queryParams, showLoader: true });
	}

	checkIfTailModeIsEnabled() {
		const tailModeConfig = localStorage.getItem('EVENT_LOGS_TAIL_MODE');
		return tailModeConfig ? JSON.parse(tailModeConfig) : false;
	}

	async getEventLogs(requestDetails?: FILTER_QUERY_PARAM) {
		if (requestDetails?.showLoader) this.isloadingEvents = true;

		try {
			const cleanedQuery = JSON.parse(JSON.stringify(requestDetails));
			delete cleanedQuery.showLoader;
			const eventsResponse = await this.eventsService.getEvents(cleanedQuery);
			this.events = eventsResponse.data;

			this.displayedEvents = await this.generalService.setContentDisplayed(eventsResponse.data.content, this.queryParams?.sort || 'desc');
			this.isloadingEvents = false;

			if (this.eventsDetailsItem) return;
			else {
				this.eventsDetailsItem = this.events?.content[0];
				if (this.eventsDetailsItem?.uid) {
					this.getEventDeliveriesForSidebar(this.eventsDetailsItem.uid);
					this.getDuplicateEvents(this.eventsDetailsItem);
				} else this.isLoadingSidebarDeliveries = false;
			}

			return eventsResponse;
		} catch (error: any) {
			this.isloadingEvents = false;
			return error;
		}
	}

	async getDuplicateEvents(event: EVENT) {
		if (!event.is_duplicate_event || !event.idempotency_key) return;

		this.isFetchingDuplicateEvents = true;
		try {
			const eventsResponse = await this.eventsService.getEvents({
				idempotencyKey: event?.idempotency_key
			});
			this.duplicateEvents = eventsResponse.data.content;
			this.isFetchingDuplicateEvents = false;
		} catch {
			this.isFetchingDuplicateEvents = false;
		}
	}

	async getEventDeliveriesForSidebar(eventId: string) {
		this.isLoadingSidebarDeliveries = true;
		this.sidebarEventDeliveries = [];

		try {
			const response = await this.eventsService.getEventDeliveries({ eventId });
			this.sidebarEventDeliveries = response.data.content;
			this.isLoadingSidebarDeliveries = false;

			return;
		} catch (error) {
			this.isLoadingSidebarDeliveries = false;
			return error;
		}
	}

	async fetchRetryCount(data: FILTER_QUERY_PARAM) {
		this.queryParams = data;

		if (!data) return;
		const page = this.route.snapshot.queryParams.page || 1;
		this.fetchingCount = true;
		try {
			const response = await this.eventsLogService.getRetryCount(data);

			this.batchRetryCount = response.data.num;
			this.fetchingCount = false;
			this.batchDialog.nativeElement.showModal();
		} catch (error) {
			this.fetchingCount = false;
		}
	}

	async replayEvent(requestDetails: { eventId: string }) {
		this.isRetrying = true;
		try {
			const response = await this.eventsLogService.retryEvent({ eventId: requestDetails.eventId });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.isRetrying = false;
			return;
		} catch (error) {
			this.isRetrying = true;
			return error;
		}
	}

	async batchReplayEvent() {
		this.isRetrying = true;

		try {
			const response = await this.eventsLogService.batchRetryEvent(this.queryParams);

			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.batchDialog.nativeElement.close();
			this.isRetrying = false;
		} catch (error) {
			this.isRetrying = false;
		}
	}

	viewSource(sourceId?: string) {
		if (!sourceId || this.portalToken) return;
		this.router.navigate([`/projects/${this.privateService.getProjectDetails?.uid}/sources/${sourceId}`]);
	}

	viewEventDeliveries(event: EVENT, filterByIdempotencyKey?: boolean) {
		const queryParams: any = {
			eventId: event.uid
		};
		if (filterByIdempotencyKey) queryParams['idempotencyKey'] = event.idempotency_key;

		this.router.navigate([`/projects/${this.privateService.getProjectDetails?.uid}/events`], { queryParams });
	}
}
