import { Component, ElementRef, OnDestroy, OnInit, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import type { ENDPOINT } from 'src/app/models/endpoint.model';
import { FILTER_QUERY_PARAM } from 'src/app/models/event.model';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { SUBSCRIPTION } from 'src/app/models/subscription';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';

@Component({
    selector: 'app-subscriptions',
    templateUrl: './subscriptions.component.html',
    styleUrls: ['./subscriptions.component.scss'],
    standalone: false
})
export class SubscriptionsComponent implements OnInit, OnDestroy {
	@ViewChild('subscriptionDialog', { static: true }) subscriptionDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('detailsDialog', { static: true }) detailsDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('deleteDialog', { static: true }) deleteDialog!: ElementRef<HTMLDialogElement>;

	activeSubscription?: SUBSCRIPTION;
	subscriptions?: { content: SUBSCRIPTION[]; pagination?: PAGINATION };
	isLoadingSubscriptions = true;
	fetchError = false;
	isDeletingSubscription = false;
	showDeleteSubscriptionModal = false;
	action: 'create' | 'update' = 'create';
	subscriptionSearchString = '';
	currentPage = 1;

	// Endpoint filter dropdown state
	filterEndpoints: ENDPOINT[] = [];
	loadingFilterEndpoints = false;
	endpointSearchString = '';
	selectedEndpointData?: ENDPOINT;

	queryParams: FILTER_QUERY_PARAM = {};

	private searchTimeout: any;
	private endpointSearchTimeout: any;

	constructor(private route: ActivatedRoute, public privateService: PrivateService, public router: Router, private generalService: GeneralService, public licenseService: LicensesService) {}

	async ngOnInit() {
		this.queryParams = { ...this.route.snapshot.queryParams };
		this.subscriptionSearchString = this.queryParams.name || '';

		const urlParam = this.route.snapshot.params.id;
		if (urlParam) {
			urlParam === 'new' ? (this.action = 'create') : (this.action = 'update');
			this.subscriptionDialog.nativeElement.showModal();
		}

		this.getEndpointsForFilter();
		await this.getSubscriptions();

		this.route.queryParams.subscribe(params => {
			this.activeSubscription = this.subscriptions?.content.find(subscription => subscription.uid === params?.id);
			if (params.id) this.detailsDialog.nativeElement.showModal();
		});
	}

	ngOnDestroy() {
		clearTimeout(this.searchTimeout);
		clearTimeout(this.endpointSearchTimeout);
	}

	get projectType(): string | undefined {
		return this.privateService.getProjectDetails?.type;
	}

	// ------- data fetching -------

	async getSubscriptions(requestDetails?: CURSOR & { name?: string; endpointId?: string; hideLoader?: boolean }) {
		const { hideLoader, ...filters } = requestDetails || {};
		this.isLoadingSubscriptions = !hideLoader;
		this.fetchError = false;

		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, ...filters });

		try {
			const subscriptionsResponse = await this.privateService.getSubscriptions(this.queryParams);
			this.subscriptions = subscriptionsResponse.data;
			this.subscriptions?.content?.length === 0 ? localStorage.setItem('isActiveProjectConfigurationComplete', 'false') : localStorage.setItem('isActiveProjectConfigurationComplete', 'true');
			this.isLoadingSubscriptions = false;
		} catch (error) {
			this.fetchError = true;
			this.isLoadingSubscriptions = false;
		}
	}

	// ------- search -------

	onSearch() {
		clearTimeout(this.searchTimeout);
		this.searchTimeout = setTimeout(() => {
			this.currentPage = 1;
			this.getSubscriptions({ name: this.subscriptionSearchString.trim(), hideLoader: true });
		}, 400);
	}

	// ------- endpoint filter -------

	async getEndpointsForFilter(search = '') {
		this.loadingFilterEndpoints = true;
		try {
			const response = await this.privateService.getEndpoints({ q: search });
			this.filterEndpoints = response.data.content || [];
			// Resolve the endpoint name when the filter came in from the URL.
			if (this.queryParams.endpointId && !this.selectedEndpointData) {
				this.selectedEndpointData = this.filterEndpoints.find(endpoint => endpoint.uid === this.queryParams.endpointId);
			}
		} catch (error) {
			this.filterEndpoints = [];
		}
		this.loadingFilterEndpoints = false;
	}

	onEndpointSearch() {
		clearTimeout(this.endpointSearchTimeout);
		this.endpointSearchTimeout = setTimeout(() => this.getEndpointsForFilter(this.endpointSearchString.trim()), 400);
	}

	updateEndpointFilter(endpoint: ENDPOINT) {
		this.selectedEndpointData = endpoint;
		this.currentPage = 1;
		this.getSubscriptions({ endpointId: endpoint.uid, hideLoader: true });
	}

	clearEndpointFilter() {
		this.selectedEndpointData = undefined;
		delete this.queryParams.endpointId;
		this.currentPage = 1;
		this.getSubscriptions({ endpointId: '', hideLoader: true });
	}

	get hasActiveFilters(): boolean {
		return !!this.subscriptionSearchString.trim() || !!this.queryParams.endpointId;
	}

	clearAllFilters() {
		this.subscriptionSearchString = '';
		this.selectedEndpointData = undefined;
		delete this.queryParams.endpointId;
		this.currentPage = 1;
		this.getSubscriptions({ name: '', endpointId: '', hideLoader: true });
	}

	// ------- presentation -------

	subscriptionCountLabel(): string {
		const total = this.subscriptions?.pagination?.total ?? this.subscriptions?.content?.length ?? 0;
		return `${total} subscription${total === 1 ? '' : 's'}`;
	}

	hasFilter(filterObject?: { headers?: Object; body?: Object }): boolean {
		if (!filterObject) return false;
		return Object.keys(filterObject.body ?? {}).length > 0 || Object.keys(filterObject.headers ?? {}).length > 0;
	}

	hasEventTypes(subscription: SUBSCRIPTION): boolean {
		return (subscription.filter_config?.event_types?.length ?? 0) > 1 || (subscription.filter_config?.event_types?.[0] ?? '*') !== '*';
	}

	copyText(text: string | undefined, label: string, event: Event) {
		event.stopPropagation();
		if (!text) return;
		navigator.clipboard?.writeText(text).then(() => {
			this.generalService.showNotification({ message: `${label} copied to clipboard`, style: 'info' });
		});
	}

	// ------- pagination -------

	paginateSubscriptions(direction: 'next' | 'prev') {
		const pagination = this.subscriptions?.pagination;
		if (!pagination) return;

		const cursor =
			direction === 'next' ? { next_page_cursor: pagination.next_page_cursor, prev_page_cursor: '', direction: 'next' as const } : { prev_page_cursor: pagination.prev_page_cursor, next_page_cursor: '', direction: 'prev' as const };

		this.currentPage = Math.max(1, this.currentPage + (direction === 'next' ? 1 : -1));
		this.getSubscriptions({ ...cursor, hideLoader: true });
	}

	get pageRangeLabel(): string {
		const contentLength = this.subscriptions?.content?.length || 0;
		if (!contentLength) return '0 subscriptions';

		const perPage = this.subscriptions?.pagination?.per_page || contentLength;
		const start = (this.currentPage - 1) * perPage + 1;
		const end = start + contentLength - 1;
		const total = this.subscriptions?.pagination?.total;

		return total ? `${start}-${end} of ${total}` : `${start}-${end}`;
	}

	// ------- dialogs / actions -------

	closeModal() {
		this.detailsDialog.nativeElement.close();
		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid + '/subscriptions');
	}

	createSubscription(action: any) {
		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid + '/subscriptions');
		if (action !== 'cancel') this.generalService.showNotification({ message: `Subscription has been ${action}d successfully`, style: 'success' });
	}

	async deleteSubscripton() {
		this.isDeletingSubscription = true;

		try {
			const response = await this.privateService.deleteSubscription(this.activeSubscription?.uid || '');
			this.generalService.showNotification({ message: response?.message, style: 'success' });
			this.getSubscriptions({ hideLoader: true });
			delete this.activeSubscription;
			this.deleteDialog.nativeElement.close();
			this.isDeletingSubscription = false;
		} catch (error) {
			this.isDeletingSubscription = false;
		}
	}
}
