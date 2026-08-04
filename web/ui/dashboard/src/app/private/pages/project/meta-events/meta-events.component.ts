import {Component, OnInit} from '@angular/core';
import {CommonModule} from '@angular/common';
import {PrivateService} from 'src/app/private/private.service';
import {MetaEventsService} from './meta-events.service';
import {GeneralService} from 'src/app/services/general/general.service';
import {TagComponent} from 'src/app/components/tag/tag.component';
import {PrismModule} from 'src/app/private/components/prism/prism.module';
import {StatusColorModule} from 'src/app/pipes/status-color/status-color.module';
import {META_EVENT} from 'src/app/models/project.model';
import {CURSOR, PAGINATION} from 'src/app/models/global.model';
import {Router} from '@angular/router';
import {PermissionDirective} from '../../../components/permission/permission.directive';

@Component({
    selector: 'convoy-meta-events',
    imports: [
        CommonModule,
        TagComponent,
        StatusColorModule,
        PrismModule,
        PermissionDirective
    ],
    templateUrl: './meta-events.component.html',
    styleUrls: ['./meta-events.component.scss']
})
export class MetaEventsComponent implements OnInit {
	isLoadingMetaEvents = false;
	isRetryingMetaEvent = false;
	fetchError = false;
	metaEvents!: { pagination: PAGINATION; content: META_EVENT[] };
	displayedMetaEvents!: { date: string; content: META_EVENT[] }[];
	selectedMetaEvent: any;
	currentPage = 1;

	constructor(public privateService: PrivateService, public generalService: GeneralService, private metaEventsService: MetaEventsService, private router: Router) {}

	ngOnInit(): void {
		this.getMetaEvents();
	}

	get isMetaEventEnabled(): Boolean {
		const isMetaEventEnabled = this.privateService.getProjectDetails?.config?.meta_event?.is_enabled || false;
		return isMetaEventEnabled;
	}

	async getMetaEvents(requestDetails?: CURSOR) {
		this.isLoadingMetaEvents = true;
		this.fetchError = false;
		try {
			const response = await this.metaEventsService.getMetaEvents(requestDetails);
			this.metaEvents = response.data;
			if (this.metaEvents?.content?.length) this.selectedMetaEvent = this.metaEvents?.content[0];
			this.displayedMetaEvents = await this.generalService.setContentDisplayed(response.data.content);
			this.isLoadingMetaEvents = false;
		} catch {
			this.fetchError = true;
			this.isLoadingMetaEvents = false;
		}
	}

	async retryMetaEvent(metaEventId: string) {
		this.isRetryingMetaEvent = true;
		try {
			const response = await this.metaEventsService.retryMetaEvent(metaEventId);
			this.isRetryingMetaEvent = false;
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.getMetaEvents();
		} catch {
			this.isRetryingMetaEvent = false;
		}
	}

	groupDateLabel(date: string): string {
		return date.replace(',', '');
	}

	paginateMetaEvents(direction: 'next' | 'prev') {
		const pagination = this.metaEvents?.pagination;
		if (!pagination) return;

		const cursor: CURSOR =
			direction === 'next' ? { next_page_cursor: pagination.next_page_cursor, prev_page_cursor: '', direction: 'next' } : { prev_page_cursor: pagination.prev_page_cursor, next_page_cursor: '', direction: 'prev' };

		this.currentPage = Math.max(1, this.currentPage + (direction === 'next' ? 1 : -1));
		this.getMetaEvents(cursor);
	}

	get pageRangeLabel(): string {
		const contentLength = this.metaEvents?.content?.length || 0;
		if (!contentLength) return '0 meta events';

		const perPage = this.metaEvents?.pagination?.per_page || contentLength;
		const start = (this.currentPage - 1) * perPage + 1;
		const end = start + contentLength - 1;
		const total = this.metaEvents?.pagination?.total;

		return total ? `${start}-${end} of ${total}` : `${start}-${end}`;
	}

	routeToMetaEvents() {
		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid + '/settings?activePage=meta events config');
	}
}
