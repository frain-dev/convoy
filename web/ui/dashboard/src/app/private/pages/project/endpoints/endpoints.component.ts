import { Component, ElementRef, OnDestroy, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { CreateEndpointComponent } from 'src/app/private/components/create-endpoint/create-endpoint.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { FormsModule } from '@angular/forms';
import { ProjectService } from '../project.service';
import { PermissionDirective } from 'src/app/private/components/permission/permission.directive';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { EndpointSecretComponent } from './endpoint-secret/endpoint-secret.component';
import { EndpointsService } from './endpoints.service';
import { LicensesService } from '../../../../services/licenses/licenses.service';
import { SettingsService } from '../../settings/settings.service';
import { UrlTemplatePartsPipe } from 'src/app/pipes/url-template-parts/url-template-parts.pipe';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';

@Component({
    selector: 'convoy-endpoints',
    imports: [
        CommonModule,
        DropdownComponent,
        DropdownOptionDirective,
        CreateEndpointComponent,
        FormsModule,
        RouterModule,
        PermissionDirective,
        EndpointSecretComponent,
        DeleteModalComponent,
        DialogDirective,
        UrlTemplatePartsPipe,
        TooltipComponent
    ],
    templateUrl: './endpoints.component.html',
    styleUrls: ['./endpoints.component.scss']
})
export class EndpointsComponent implements OnInit, OnDestroy {
	@ViewChild('endpointDialog', { static: true }) endpointDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('secretDialog', { static: true }) secretDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('deleteDialog', { static: true }) deleteDialog!: ElementRef<HTMLDialogElement>;

	showCreateEndpointModal = this.router.url.split('/')[4] === 'new';
	showEditEndpointModal = this.router.url.split('/')[5] === 'edit';
	// The failure-rate column covers a fixed window (failureRateWindowDays), named in the
	// header so the covered range is never hidden. Circuit breaker state is reflected on
	// the Status column instead of a separate live lens.
	// Fixed window (days) for the failure-rate column.
	readonly failureRateWindowDays = 30;
	// The circuit breaker rolling rate covers the project's observability window
	// (minutes). Default mirrors the server default when a project has no explicit
	// circuit_breaker config. Used only for the tripped-breaker status tag tooltip.
	failureRateWindow = 5;
	// Status tag tooltip panel: above the tag, left edge pinned to the tag so the
	// panel grows to the right; a centered/right-anchored panel overflows this
	// left-hugging column off the viewport.
	readonly statusTooltipClass = '!min-w-[280px] !left-0 !translate-x-0 after:!left-[24px] after:!translate-x-0';
	endpoints?: { pagination?: PAGINATION; content?: ENDPOINT[] };
	selectedEndpoint?: ENDPOINT;
	isLoadingEndpoints = true;
	fetchError = false;
	isDeletingEndpoint = false;
	showDeleteModal = false;
	isTogglingEndpoint = false;
	isSendingTestEvent = false;
	endpointSearchString = '';
	statusFilter = '';
	readonly endpointStatuses = ['active', 'paused', 'inactive'];
	currentPage = 1;
	action: 'create' | 'update' = 'create';
	endpointURLTemplatesFeatureEnabled = false;
	private featureFlagReady?: Promise<void>;
	private searchTimeout: any;

	constructor(public router: Router, public privateService: PrivateService, public projectService: ProjectService, private endpointService: EndpointsService, private generalService: GeneralService, public route: ActivatedRoute, public licenseService: LicensesService, private settingsService: SettingsService) {}

	ngOnInit() {
		const urlParam = this.route.snapshot.params.id;
		if (urlParam) {
			urlParam === 'new' ? (this.action = 'create') : (this.action = 'update');
			this.endpointDialog.nativeElement.showModal();
		}
		this.failureRateWindow = this.privateService.getProjectDetails?.config?.circuit_breaker?.observability_window || 5;

		this.featureFlagReady = this.checkEndpointURLTemplatesFeatureFlag();
		this.getEndpoints();
	}

	ngOnDestroy() {
		clearTimeout(this.searchTimeout);
	}

	// Fixed window for the failure-rate column.
	private get failureRateRange(): { startDate: string; endDate: string } {
		const end = new Date();
		const start = new Date(end.getTime() - this.failureRateWindowDays * 24 * 60 * 60 * 1000);
		// Match the backend date format (yyyy-MM-ddTHH:mm:ss, no timezone).
		return { startDate: start.toISOString().slice(0, -5), endDate: end.toISOString().slice(0, -5) };
	}

	// A tripped breaker (open or half-open) overrides the status tag only while the
	// endpoint is otherwise active. A persisted inactive/paused status outranks a
	// lingering open breaker in Redis: deliveries will not resume on breaker cooldown
	// alone, so the tag must surface the Activate/Unpause guidance instead. The server
	// only attaches cb_state after its own license + org-flag gate, so a non-null value
	// is trusted as-is; re-gating here on a separate flags request would hide a
	// genuinely tripped breaker whenever that request fails.
	circuitBreakerOpen(endpoint: ENDPOINT): boolean {
		if (endpoint.status !== 'active') return false;
		return endpoint.cb_state === 'open' || endpoint.cb_state === 'half-open';
	}

	// First tooltip line for the failure-rate pill: the delivery stats for the fixed
	// window. Retrying deliveries count as failures-so-far (they have failed at least
	// once); the static exclusions line lives in the template.
	periodFailureRateStats(endpoint?: ENDPOINT): string {
		const range = `the last ${this.failureRateWindowDays} days`;
		if (!endpoint || endpoint.period_failure_rate === null || endpoint.period_failure_rate === undefined) {
			return `No delivered events in ${range}.`;
		}
		const success = endpoint.success_count ?? 0;
		const failure = endpoint.failure_count ?? 0;
		const retry = endpoint.retry_count ?? 0;
		const retrying = retry > 0 ? `, ${retry} retrying` : '';
		return `${success} successful, ${failure} failed${retrying} over ${range}.`;
	}

	// First tooltip line for the circuit-breaker status tag: state + the breaker's
	// rolling rate over the project's observability window (not the 30d column rate).
	// The muted explanation line comes from cbStatusTooltipDetail.
	cbStatusTooltip(endpoint: ENDPOINT): string {
		const rate = Math.round(endpoint.failure_rate ?? 0);
		const state = endpoint.cb_state === 'half-open' ? 'recovering' : 'open';
		return `Circuit breaker is ${state}: deliveries failed at ${rate}% over the last ${this.failureRateWindow}m.`;
	}

	// Second (muted) tooltip line: what the breaker is doing in this state.
	cbStatusTooltipDetail(endpoint: ENDPOINT): string {
		if (endpoint.cb_state === 'half-open') {
			return 'Convoy is probing the endpoint and resumes deliveries once probes succeed.';
		}
		return 'Deliveries are paused; Convoy retries after a cooldown.';
	}

	// Tooltip for the plain (non-breaker) status tag.
	statusTooltip(endpoint: ENDPOINT): string {
		switch (endpoint.status) {
			case 'inactive':
				return 'Convoy deactivated this endpoint after sustained delivery failures. New deliveries are discarded.';
			case 'paused':
				return 'Deliveries to this endpoint are paused.';
			default:
				return 'Endpoint is receiving deliveries normally.';
		}
	}

	// Second (muted) line: how to get out of the state. Empty when no action is needed.
	statusTooltipDetail(endpoint: ENDPOINT): string {
		switch (endpoint.status) {
			case 'inactive':
				return 'Fix the endpoint, then use Activate Endpoint in the menu to resume deliveries.';
			case 'paused':
				return 'Use Unpause in the menu to resume deliveries.';
			default:
				return '';
		}
	}

	// ------- table data / presentation -------

	get filteredEndpoints(): ENDPOINT[] {
		const content = this.endpoints?.content || [];
		if (!this.statusFilter) return content;
		return content.filter(endpoint => endpoint.status === this.statusFilter);
	}

	get hasActiveFilters(): boolean {
		return !!this.statusFilter || !!this.endpointSearchString;
	}

	endpointCountLabel(): string {
		const total = this.endpoints?.pagination?.total ?? this.endpoints?.content?.length ?? 0;
		return `${total} endpoint${total === 1 ? '' : 's'}`;
	}

	statusPillLabel(endpoint: ENDPOINT): string {
		if (this.circuitBreakerOpen(endpoint)) return endpoint.cb_state === 'half-open' ? 'Recovering' : 'Failing';
		return endpoint.status ? endpoint.status.charAt(0).toUpperCase() + endpoint.status.slice(1) : '-';
	}

	statusPillClass(endpoint: ENDPOINT): string {
		if (this.circuitBreakerOpen(endpoint)) return 'bg-[#F5C6C6]';
		switch (endpoint.status) {
			case 'active':
				return 'bg-[#C6F5CE]';
			case 'paused':
				return 'bg-[#E3EBF2]';
			case 'inactive':
				return 'bg-[#F5C6C6]';
			default:
				return 'bg-new.surface-muted';
		}
	}

	failureRateLabel(endpoint: ENDPOINT): string {
		if (endpoint.period_failure_rate === null || endpoint.period_failure_rate === undefined) return '-';
		const rate = endpoint.period_failure_rate * 100;
		return `${Number.isInteger(rate) ? rate : rate.toFixed(2)}%`;
	}

	copyText(text: string | undefined, label: string, event: Event) {
		event.stopPropagation();
		if (!text) return;
		navigator.clipboard?.writeText(text).then(() => {
			this.generalService.showNotification({ message: `${label} copied to clipboard`, style: 'info' });
		});
	}

	// ------- data fetching -------

	async getEndpoints(requestDetails?: CURSOR & { search?: string; hideLoader?: boolean }) {
		this.isLoadingEndpoints = !requestDetails?.hideLoader;
		this.fetchError = false;

		const range = this.failureRateRange;
		try {
			const response = await this.privateService.getEndpoints({
				...requestDetails,
				q: requestDetails?.search ?? this.endpointSearchString,
				startDate: range.startDate,
				endDate: range.endDate
			});
			this.endpoints = response.data;
			this.isLoadingEndpoints = false;
		} catch {
			this.fetchError = true;
			this.isLoadingEndpoints = false;
		}
	}

	onSearch() {
		clearTimeout(this.searchTimeout);
		this.searchTimeout = setTimeout(() => {
			this.currentPage = 1;
			this.getEndpoints({ search: this.endpointSearchString, hideLoader: true });
		}, 400);
	}

	setStatusFilter(status = '') {
		this.statusFilter = status;
	}

	clearAllFilters() {
		this.statusFilter = '';
		if (this.endpointSearchString) {
			this.endpointSearchString = '';
			this.currentPage = 1;
			this.getEndpoints({ hideLoader: true });
		}
	}

	// ------- pagination -------

	paginateEndpoints(direction: 'next' | 'prev') {
		const pagination = this.endpoints?.pagination;
		if (!pagination) return;

		const cursor =
			direction === 'next' ? { next_page_cursor: pagination.next_page_cursor, prev_page_cursor: '', direction: 'next' as const } : { prev_page_cursor: pagination.prev_page_cursor, next_page_cursor: '', direction: 'prev' as const };

		this.currentPage = Math.max(1, this.currentPage + (direction === 'next' ? 1 : -1));
		this.getEndpoints({ ...cursor, hideLoader: true });
	}

	get pageRangeLabel(): string {
		const contentLength = this.endpoints?.content?.length || 0;
		if (!contentLength) return '0 endpoints';

		const perPage = this.endpoints?.pagination?.per_page || contentLength;
		const start = (this.currentPage - 1) * perPage + 1;
		const end = start + contentLength - 1;
		const total = this.endpoints?.pagination?.total;

		return total ? `${start}-${end} of ${total}` : `${start}-${end}`;
	}

	// ------- feature flags -------

	async checkEndpointURLTemplatesFeatureFlag() {
		// Only the org-scoped early-adopter feature flag is checked here; the license
		// side is verified separately in sendTestEvent. Both must hold for the backend
		// to run template matching, so we mirror that before using the dynamic path.
		const org = localStorage.getItem('CONVOY_ORG');
		if (!org) return;
		try {
			this.endpointURLTemplatesFeatureEnabled = await this.settingsService.checkFeatureFlagEnabled({
				org_id: JSON.parse(org).uid,
				feature_key: 'endpoint-url-templates'
			});
		} catch {
			this.endpointURLTemplatesFeatureEnabled = false;
		}
	}

	// ------- endpoint actions -------

	async deleteEndpoint() {
		if (!this.selectedEndpoint) return;
		this.isDeletingEndpoint = true;

		try {
			const response = await this.endpointService.deleteEndpoint(this.selectedEndpoint?.uid || '');
			this.getEndpoints({ hideLoader: true });

			this.generalService.showNotification({ style: 'success', message: response.message });
			this.deleteDialog.nativeElement.close();
			this.isDeletingEndpoint = false;
		} catch {
			this.isDeletingEndpoint = false;
		}
	}

	async toggleEndpoint() {
		this.isTogglingEndpoint = true;
		if (!this.selectedEndpoint?.uid) return;

		try {
			const response = await this.endpointService.toggleEndpoint(this.selectedEndpoint?.uid);
			this.endpoints?.content?.forEach(endpoint => {
				if (response.data.uid === endpoint.uid) endpoint.status = response.data.status;
			});
			this.generalService.showNotification({ message: `${this.selectedEndpoint?.name} status updated successfully`, style: 'success' });
			this.isTogglingEndpoint = false;
		} catch {
			this.isTogglingEndpoint = false;
		}
	}

	async activateEndpoint() {
		this.isTogglingEndpoint = true;
		if (!this.selectedEndpoint?.uid) return;

		try {
			const response = await this.endpointService.activateEndpoint(this.selectedEndpoint?.uid);
			// Patch the cached row from the response first so the tag can't show stale
			// state even if the refetch below fails. Activation also resets the circuit
			// breaker server-side, so cb_state is cleared alongside status.
			this.endpoints?.content?.forEach(endpoint => {
				if (response.data.uid === endpoint.uid) {
					endpoint.status = response.data.status;
					endpoint.cb_state = null;
				}
			});
			if (this.selectedEndpoint?.uid === response.data.uid) this.selectedEndpoint = { ...this.selectedEndpoint, status: response.data.status, cb_state: null };
			this.generalService.showNotification({ message: `${this.selectedEndpoint?.name} activated successfully`, style: 'success' });
			this.isTogglingEndpoint = false;
			// Refetch for full server truth (counts, rates, breaker sample).
			await this.getEndpoints({ hideLoader: true });
		} catch {
			this.isTogglingEndpoint = false;
		}
	}

	async sendTestEvent() {
		// Under the dashboard version header the endpoint URL comes back as target_url,
		// so read both before deciding there is nothing to test against.
		const url = this.selectedEndpoint?.url || this.selectedEndpoint?.target_url;
		if (!url) {
			this.generalService.showNotification({ message: 'Endpoint has no URL to test against', style: 'error' });
			return;
		}

		const data = { data: 'test event from Convoy', convoy: 'https://getconvoy.io', amount: 1000 };

		// Only templated endpoints (e.g. /tx/{reference}/callback) use the dynamic path:
		// it resolves the concrete URL against the endpoint template and bypasses
		// subscription event-type filters. Concrete endpoints keep the endpoint-bound
		// path so the test stays tied to the selected endpoint (its secrets, auth and
		// state); routing them through dynamic would bind by URL match and could
		// auto-create an orphan endpoint when the URL does not match exactly.
		//
		// The dynamic worker only runs template matching when both the license and the
		// org feature flag are on. If either is off it skips the lookup and mints a new
		// orphan endpoint for the URL, so we require both here and otherwise fall back
		// to the endpoint-bound path.
		const isTemplated = /\{[A-Za-z_][A-Za-z0-9_]*\}/.test(url);

		this.isSendingTestEvent = true;
		try {
			// Wait for the feature flag check kicked off in ngOnInit so a fast click does
			// not misroute a templated endpoint to the endpoint-bound path (which cannot
			// resolve the template) just because the check is still in flight.
			await this.featureFlagReady;
			const useDynamic = isTemplated && this.endpointURLTemplatesFeatureEnabled && this.licenseService.hasLicense('EndpointURLTemplates');

			// For the templated path, substitute each {token} with a dummy value so the
			// URL is concrete; event_types (plural) is intentionally omitted so the
			// endpoint's real subscription filter is not overwritten.
			const response = useDynamic
				? await this.endpointService.sendDynamicEvent({
						body: { url: url.replace(/\{[A-Za-z_][A-Za-z0-9_]*\}/g, () => this.generateTestToken()), data, event_type: 'test.convoy' }
				  })
				: await this.endpointService.sendEvent({ body: { data, endpoint_id: this.selectedEndpoint?.uid, event_type: 'test.convoy' } });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.isSendingTestEvent = false;
		} catch {
			this.isSendingTestEvent = false;
		}
	}

	private generateTestToken(): string {
		if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
			return crypto.randomUUID().replace(/-/g, '');
		}
		return `test${Date.now()}${Math.random().toString(36).slice(2)}`;
	}

	viewSubscription() {
        this.router.navigate([`/projects/${this.privateService.getProjectDetails?.uid}/subscriptions`], { queryParams: { endpointId: this.selectedEndpoint?.uid || '' } });
	}

	cancel() {
		this.endpointDialog.nativeElement.close();
		this.router.navigateByUrl('/projects/' + this.projectService.activeProjectDetails?.uid + '/endpoints');
	}

	protected readonly Math = Math;
}
