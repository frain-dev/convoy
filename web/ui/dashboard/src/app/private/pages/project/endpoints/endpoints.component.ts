import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { CardComponent } from 'src/app/components/card/card.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { DialogDirective, DialogHeaderComponent } from 'src/app/components/dialog/dialog.directive';
import { CreateEndpointComponent } from 'src/app/private/components/create-endpoint/create-endpoint.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { FormsModule } from '@angular/forms';
import { TableComponent, TableCellComponent, TableRowComponent, TableHeadCellComponent, TableHeadComponent } from 'src/app/components/table/table.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { ProjectService } from '../project.service';
import { PaginationComponent } from 'src/app/private/components/pagination/pagination.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { PermissionDirective } from 'src/app/private/components/permission/permission.directive';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { EndpointSecretComponent } from './endpoint-secret/endpoint-secret.component';
import { EndpointsService } from './endpoints.service';
import { LoaderModule } from 'src/app/private/components/loader/loader.module';
import { LicensesService } from '../../../../services/licenses/licenses.service';
import { SettingsService } from '../../settings/settings.service';
import { UrlTemplatePartsPipe } from 'src/app/pipes/url-template-parts/url-template-parts.pipe';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';

@Component({
    selector: 'convoy-endpoints',
    imports: [
        CommonModule,
        ButtonComponent,
        TableCellComponent,
        TableHeadComponent,
        TableHeadCellComponent,
        TableRowComponent,
        TableCellComponent,
        TableComponent,
        CardComponent,
        EmptyStateComponent,
        DropdownComponent,
        DropdownOptionDirective,
        DialogHeaderComponent,
        CreateEndpointComponent,
        TagComponent,
        FormsModule,
        RouterModule,
        StatusColorModule,
        PaginationComponent,
        CopyButtonComponent,
        PermissionDirective,
        EndpointSecretComponent,
        DeleteModalComponent,
        LoaderModule,
        DialogDirective,
        UrlTemplatePartsPipe,
        TooltipComponent
    ],
    templateUrl: './endpoints.component.html',
    styleUrls: ['./endpoints.component.scss']
})
export class EndpointsComponent implements OnInit {
	@ViewChild('endpointDialog', { static: true }) endpointDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('secretDialog', { static: true }) secretDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('deleteDialog', { static: true }) deleteDialog!: ElementRef<HTMLDialogElement>;

	showCreateEndpointModal = this.router.url.split('/')[4] === 'new';
	showEditEndpointModal = this.router.url.split('/')[5] === 'edit';
	// The failure-rate column covers a fixed window (failureRateWindowDays), named in the
	// header so the covered range is never hidden. Circuit breaker state is reflected on
	// the Status column instead of a separate live lens.
	endpointsTableHead = ['Name', 'Status', 'Url', 'ID', 'Failure rate (30d)', ''];
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
	displayedEndpoints?: { date: string; content: ENDPOINT[] }[];
	endpoints?: { pagination?: PAGINATION; content?: ENDPOINT[] };
	selectedEndpoint?: ENDPOINT;
	isLoadingEndpoints = true;
	isDeletingEndpoint = false;
	showDeleteModal = false;
	isTogglingEndpoint = false;
	isSendingTestEvent = false;
	endpointSearchString!: string;
	action: 'create' | 'update' = 'create';
	userSearch = false;
	endpointURLTemplatesFeatureEnabled = false;
	private featureFlagReady?: Promise<void>;

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

	async getEndpoints(requestDetails?: CURSOR & { search?: string; hideLoader?: boolean }) {
		this.isLoadingEndpoints = !requestDetails?.hideLoader;
		this.userSearch = !!requestDetails?.search;

		const range = this.failureRateRange;
		try {
			const response = await this.privateService.getEndpoints({
				...requestDetails,
				q: requestDetails?.search || this.endpointSearchString,
				startDate: range.startDate,
				endDate: range.endDate
			});
			this.endpoints = response.data;
			if (response.data.content) this.displayedEndpoints = this.generalService.setContentDisplayed(response.data.content, 'desc');
			this.isLoadingEndpoints = false;
		} catch {
			this.isLoadingEndpoints = false;
		}
	}

	searchEndpoint(searchDetails: { searchInput?: any }) {
		const searchString: string = searchDetails?.searchInput?.target?.value || this.endpointSearchString;
		this.getEndpoints({ search: searchString, hideLoader: true });
	}

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
			this.displayedEndpoints?.forEach(item => {
				item.content.forEach(endpoint => {
					if (response.data.uid === endpoint.uid) endpoint.status = response.data.status;
				});
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
			this.displayedEndpoints?.forEach(item => {
				item.content.forEach(endpoint => {
					if (response.data.uid === endpoint.uid) {
						endpoint.status = response.data.status;
						endpoint.cb_state = null;
					}
				});
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
