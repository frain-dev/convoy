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
	endpointsTableHead = ['Name', 'Status', 'Url', 'ID', '', '', ''];
	// Failure Rate is the circuit breaker's rolling rate over the project's
	// observability window (minutes), not an all-time rate. Default mirrors the
	// server default when a project has no explicit circuit_breaker config.
	failureRateWindow = 5;
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

	constructor(public router: Router, public privateService: PrivateService, public projectService: ProjectService, private endpointService: EndpointsService, private generalService: GeneralService, public route: ActivatedRoute, public licenseService: LicensesService, private settingsService: SettingsService) {}

	ngOnInit() {
		const urlParam = this.route.snapshot.params.id;
		if (urlParam) {
			urlParam === 'new' ? (this.action = 'create') : (this.action = 'update');
			this.endpointDialog.nativeElement.showModal();
		}
		this.failureRateWindow = this.privateService.getProjectDetails?.config?.circuit_breaker?.observability_window || 5;
		this.updateFailureRateColumnHeader();

		this.featureFlagReady = this.checkEndpointURLTemplatesFeatureFlag();
		this.checkCircuitBreakerFeatureFlag();
		this.getEndpoints();
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
		this.updateFailureRateColumnHeader();
	}

	private updateFailureRateColumnHeader() {
		const show = this.licenseService.hasLicense('CircuitBreaking') && this.circuitBreakerFeatureEnabled;
		this.endpointsTableHead[4] = show ? `Failure Rate (last ${this.failureRateWindow}m)` : '';
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

		try {
			const response = await this.privateService.getEndpoints({ ...requestDetails, q: requestDetails?.search || this.endpointSearchString });
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
