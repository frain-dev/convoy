import { Component, ElementRef, OnDestroy, OnInit, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { SOURCE } from 'src/app/models/source.model';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { SourcesService } from './sources.service';
import { PROJECT } from 'src/app/models/project.model';

@Component({
    selector: 'app-sources',
    templateUrl: './sources.component.html',
    styleUrls: ['./sources.component.scss'],
    standalone: false
})
export class SourcesComponent implements OnInit, OnDestroy {
	@ViewChild('sourceDialog', { static: true }) sourceDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('deleteDialog', { static: true }) deleteDialog!: ElementRef<HTMLDialogElement>;

	activeSource?: SOURCE;
	sources: { content: SOURCE[]; pagination?: PAGINATION } = { content: [], pagination: undefined };
	isLoadingSources = false;
	fetchError = false;
	isDeletingSource = false;
	sourceSearchString = '';
	currentPage = 1;
	projectDetails?: PROJECT;
	action: 'create' | 'update' = 'create';
	private searchTimeout: any;

	constructor(private route: ActivatedRoute, public router: Router, private sourcesService: SourcesService, public privateService: PrivateService, private generalService: GeneralService) {}

	ngOnInit() {
		this.getSources();

		const urlParam = this.route.snapshot.params.id;
		if (urlParam) {
			urlParam === 'new' ? (this.action = 'create') : (this.action = 'update');
			this.sourceDialog.nativeElement.showModal();
		}
	}

	ngOnDestroy() {
		clearTimeout(this.searchTimeout);
	}

	async getSources(requestDetails?: CURSOR & { q?: string; hideLoader?: boolean }) {
		this.isLoadingSources = !requestDetails?.hideLoader;
		this.fetchError = false;

		try {
			const sourcesResponse = await this.privateService.getSources({ ...requestDetails, q: requestDetails?.q ?? this.sourceSearchString });
			this.sources = sourcesResponse.data;
			this.isLoadingSources = false;
		} catch {
			this.fetchError = true;
			this.isLoadingSources = false;
		}
	}

	onSearch() {
		clearTimeout(this.searchTimeout);
		this.searchTimeout = setTimeout(() => {
			this.currentPage = 1;
			this.getSources({ hideLoader: true });
		}, 400);
	}

	clearSearch() {
		this.sourceSearchString = '';
		this.currentPage = 1;
		this.getSources({ hideLoader: true });
	}

	// ------- presentation -------

	sourceCountLabel(): string {
		const total = this.sources?.pagination?.total ?? this.sources?.content?.length ?? 0;
		return `${total} source${total === 1 ? '' : 's'}`;
	}

	copyText(text: string | undefined, label: string, event: Event) {
		event.stopPropagation();
		if (!text) return;
		navigator.clipboard?.writeText(text).then(() => {
			this.generalService.showNotification({ message: `${label} copied to clipboard`, style: 'info' });
		});
	}

	// ------- pagination -------

	paginateSources(direction: 'next' | 'prev') {
		const pagination = this.sources?.pagination;
		if (!pagination) return;

		const cursor =
			direction === 'next' ? { next_page_cursor: pagination.next_page_cursor, prev_page_cursor: '', direction: 'next' as const } : { prev_page_cursor: pagination.prev_page_cursor, next_page_cursor: '', direction: 'prev' as const };

		this.currentPage = Math.max(1, this.currentPage + (direction === 'next' ? 1 : -1));
		this.getSources({ ...cursor, hideLoader: true });
	}

	get pageRangeLabel(): string {
		const contentLength = this.sources?.content?.length || 0;
		if (!contentLength) return '0 sources';

		const perPage = this.sources?.pagination?.per_page || contentLength;
		const start = (this.currentPage - 1) * perPage + 1;
		const end = start + contentLength - 1;
		const total = this.sources?.pagination?.total;

		return total ? `${start}-${end} of ${total}` : `${start}-${end}`;
	}

	// ------- actions -------

	async deleteSource() {
		this.isDeletingSource = true;
		try {
			await this.sourcesService.deleteSource(this.activeSource?.uid);
			this.isDeletingSource = false;
			this.getSources({ hideLoader: true });
			this.closeModal();
			this.deleteDialog.nativeElement.close();
			this.activeSource = undefined;
		} catch (error) {
			this.isDeletingSource = false;
		}
	}

	closeCreateSourceModal(source: { action: string; data?: any }) {
		if (source.action !== 'close') this.generalService.showNotification({ message: `Source ${source.action}d successfully`, style: 'success' });
		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid + '/sources');
	}

	closeModal() {
		this.router.navigate([], { queryParams: {} });
	}
}
