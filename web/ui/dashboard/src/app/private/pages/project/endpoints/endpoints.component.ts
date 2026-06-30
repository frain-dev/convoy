import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ENDPOINT, ENDPOINT_STATS } from 'src/app/models/endpoint.model';
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
        UrlTemplatePartsPipe
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
	// Column 4 is a single "Failure rate" column. Its period is chosen from a popup on
	// the header (see failureRatePeriod). Rendered specially in the template, so the
	// string here is just a placeholder/spacer.
	endpointsTableHead = ['Name', 'Status', 'Url', 'ID', 'Failure rate', '', ''];
	// The circuit breaker rolling rate covers the project's observability window
	// (minutes). Default mirrors the server default when a project has no explicit
	// circuit_breaker config.
	failureRateWindow = 5;
	// Period for the single failure-rate column, chosen from the header popup. 'live' is
	// the circuit breaker rolling rate over the observability window (only when CB is
	// licensed + enabled); the others are the history rate computed over that window.
	failureRatePeriod: 'live' | '24h' | '7d' | '30d' = 'live';
	// hours is null for the live (circuit breaker) lens; the others scope the history rate.
	failureRateListPeriods: { uid: 'live' | '24h' | '7d' | '30d'; hours: number | null }[] = [
		{ uid: 'live', hours: null },
		{ uid: '24h', hours: 24 },
		{ uid: '7d', hours: 24 * 7 },
		{ uid: '30d', hours: 24 * 30 }
	];
	// Inline reliability detail: one row expands at a time on tap. The period is derived
	// from the header popup (statsHistoryPeriod); the period failure rate is independent
	// of the circuit breaker, while recent_failure_rate (when present) drives the
	// "currently failing" badge.
	expandedEndpointId?: string;
	expandedStats?: ENDPOINT_STATS;
	isLoadingStats = false;
	// Monotonic token so only the latest stats request applies; guards against a slower
	// earlier response (different row or period) overwriting the panel.
	private statsRequestToken = 0;
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
	// Mirrors the backend's org-scoped circuit-breaker feature flag. The Failure Rate
	// column is only meaningful when this is enabled (and licensed); otherwise the
	// backend never computes a rate and the column would show a misleading 0%.
	circuitBreakerFeatureEnabled = false;
	private featureFlagReady?: Promise<void>;
	private readonly failureRatePeriodStorageKey = 'CONVOY_ENDPOINT_FAILURE_RATE_PERIOD';

	constructor(public router: Router, public privateService: PrivateService, public projectService: ProjectService, private endpointService: EndpointsService, private generalService: GeneralService, public route: ActivatedRoute, public licenseService: LicensesService, private settingsService: SettingsService) {}

	ngOnInit() {
		const urlParam = this.route.snapshot.params.id;
		if (urlParam) {
			urlParam === 'new' ? (this.action = 'create') : (this.action = 'update');
			this.endpointDialog.nativeElement.showModal();
		}
		this.failureRateWindow = this.privateService.getProjectDetails?.config?.circuit_breaker?.observability_window || 5;
		this.restoreFailureRatePeriod();

		this.featureFlagReady = this.checkEndpointURLTemplatesFeatureFlag();
		this.checkCircuitBreakerFeatureFlag();
		this.getEndpoints();
	}

	// Restore the last failure-rate period from local storage so the column lens persists
	// across visits. getEndpoints derives the range from the effective period, so no date
	// seeding is needed here.
	private restoreFailureRatePeriod() {
		const stored = localStorage.getItem(this.failureRatePeriodStorageKey);
		if (stored === 'live' || stored === '24h' || stored === '7d' || stored === '30d') {
			this.failureRatePeriod = stored;
		}
	}

	async checkCircuitBreakerFeatureFlag() {
		// The Failure Rate column requires the org-scoped circuit-breaker flag, mirroring
		// the backend read gate (CanAccessOrgFeature). Without it the backend returns a
		// null rate, so showing the column would surface a misleading 0%. Fail closed
		// (hide the column) on any error.
		const org = localStorage.getItem('CONVOY_ORG');
		if (!org) return;
		try {
			const response = await this.settingsService.getOrganisationFeatureFlags({ org_id: JSON.parse(org).uid });
			const featureFlags = response.data || {};
			this.circuitBreakerFeatureEnabled = featureFlags['circuit-breaker'] || false;
		} catch {
			this.circuitBreakerFeatureEnabled = false;
		}
	}

	// The live (circuit breaker) period is only available when CB is licensed + enabled;
	// otherwise the backend never computes that rolling rate, so it's hidden from the popup.
	get canUseLiveFailureRate(): boolean {
		return this.licenseService.hasLicense('CircuitBreaking') && this.circuitBreakerFeatureEnabled;
	}

	// Periods offered in the header popup. Drops 'live' when the circuit breaker is not
	// available so we never offer a rate the backend won't compute.
	get availableFailureRatePeriods(): { uid: 'live' | '24h' | '7d' | '30d'; hours: number | null }[] {
		return this.failureRateListPeriods.filter(p => p.uid !== 'live' || this.canUseLiveFailureRate);
	}

	// Period actually used by the column. Falls back to 7d if 'live' is selected but CB
	// is unavailable (e.g. license/flag resolved off after the default was set).
	get effectiveFailureRatePeriod(): 'live' | '24h' | '7d' | '30d' {
		return this.failureRatePeriod === 'live' && !this.canUseLiveFailureRate ? '7d' : this.failureRatePeriod;
	}

	failureRatePeriodLabel(uid: 'live' | '24h' | '7d' | '30d'): string {
		switch (uid) {
			case 'live':
				return `Live (${this.failureRateWindow}m)`;
			case '24h':
				return 'Last 24 hours';
			case '7d':
				return 'Last 7 days';
			case '30d':
				return 'Last 30 days';
		}
	}

	// History period the inline reliability detail covers. Mirrors the header popup; the
	// 'live' lens has no range, so the historical counts default to the last 7 days while
	// the live rate is still shown separately.
	get statsHistoryPeriod(): '24h' | '7d' | '30d' {
		const p = this.effectiveFailureRatePeriod;
		return p === 'live' ? '7d' : p;
	}

	// Short period suffix shown in the column header, e.g. "(7d)". Empty for the default
	// Live lens so the header stays just "Failure rate".
	get failureRateHeaderSuffix(): string {
		const p = this.effectiveFailureRatePeriod;
		return p === 'live' ? '' : ` (${p})`;
	}

	// Rolling date range for a historical period; null for 'live' (no range, the column
	// shows the circuit breaker rate). Single source of truth so the list column and the
	// inline detail use the exact same window for a given period.
	private periodToRange(period: 'live' | '24h' | '7d' | '30d'): { startDate: string; endDate: string } | null {
		if (period === 'live') return null;
		const hours = this.failureRateListPeriods.find(p => p.uid === period)?.hours ?? 24 * 7;
		const end = new Date();
		const start = new Date(end.getTime() - hours * 60 * 60 * 1000);
		// Match the backend date format (yyyy-MM-ddTHH:mm:ss, no timezone).
		return { startDate: start.toISOString().slice(0, -5), endDate: end.toISOString().slice(0, -5) };
	}

	selectFailureRatePeriod(uid: 'live' | '24h' | '7d' | '30d') {
		this.failureRatePeriod = uid;
		localStorage.setItem(this.failureRatePeriodStorageKey, uid);
		// Refetch the list; getEndpoints derives the range from the effective period, so a
		// 'live' fallback to 7d (CB unavailable) sends the same window the panel uses.
		this.getEndpoints({ hideLoader: true });
		// Keep an open inline detail in sync with the chosen period.
		if (this.expandedEndpointId) this.getEndpointStats(this.expandedEndpointId, this.statsHistoryPeriod);
	}

	// Tap a row to expand its reliability detail; tap again to collapse.
	toggleEndpointDetails(endpoint: ENDPOINT) {
		this.selectedEndpoint = endpoint;
		if (this.expandedEndpointId === endpoint.uid) {
			this.expandedEndpointId = undefined;
			this.expandedStats = undefined;
			return;
		}
		this.expandedEndpointId = endpoint.uid;
		this.getEndpointStats(endpoint.uid, this.statsHistoryPeriod);
	}

	async getEndpointStats(endpointId: string, period: '24h' | '7d' | '30d') {
		const range = this.periodToRange(period)!;
		// Token + endpoint guard: ignore a response if a newer request started or the row
		// was collapsed/changed, so a slow earlier fetch can't overwrite the panel.
		const token = ++this.statsRequestToken;

		this.isLoadingStats = true;
		this.expandedStats = undefined;
		try {
			const response = await this.endpointService.getEndpointStats(endpointId, range);
			if (token !== this.statsRequestToken || this.expandedEndpointId !== endpointId) return;
			this.expandedStats = response.data;
		} catch {
			// Failure policy: stats are read-only decoration, so on error leave them unset
			// (the panel shows a dash) rather than surfacing an error.
			if (token !== this.statsRequestToken || this.expandedEndpointId !== endpointId) return;
			this.expandedStats = undefined;
		} finally {
			if (token === this.statsRequestToken) this.isLoadingStats = false;
		}
	}

	// Tooltip for the history failure rate column, naming the selected period and that it
	// counts only delivered attempts (success + failure), excluding discarded and in-flight.
	periodFailureRateTooltip(endpoint?: ENDPOINT): string {
		const range = `the ${this.failureRatePeriodLabel(this.effectiveFailureRatePeriod).toLowerCase()}`;
		if (!endpoint || endpoint.period_failure_rate === null || endpoint.period_failure_rate === undefined) {
			return `No delivered events in ${range}. Failure rate counts only delivered attempts (success + failure), excluding discarded and in-flight deliveries.`;
		}
		const success = endpoint.success_count ?? 0;
		const failure = endpoint.failure_count ?? 0;
		return `${failure} failed of ${success + failure} delivered (success + failure) over ${range}. Excludes discarded and in-flight deliveries.`;
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

		const range = this.periodToRange(this.effectiveFailureRatePeriod);
		try {
			const response = await this.privateService.getEndpoints({
				...requestDetails,
				q: requestDetails?.search || this.endpointSearchString,
				startDate: range?.startDate,
				endDate: range?.endDate
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
			this.displayedEndpoints?.forEach(item => {
				item.content.forEach(endpoint => {
					if (response.data.uid === endpoint.uid) endpoint.status = response.data.status;
				});
			});
			this.generalService.showNotification({ message: `${this.selectedEndpoint?.name} activated successfully`, style: 'success' });
			this.isTogglingEndpoint = false;
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
