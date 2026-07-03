import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { Router } from '@angular/router';
import { ORGANIZATION_DATA } from '../models/organisation.model';
import { GeneralService } from '../services/general/general.service';
import { PrivateService } from './private.service';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { JwtHelperService } from '@auth0/angular-jwt';
import { differenceInSeconds } from 'date-fns';
import { Subscription } from 'rxjs';
import { LicensesService } from '../services/licenses/licenses.service';
import { RbacService } from '../services/rbac/rbac.service';
import { TrialStatusService } from '../services/trial-status/trial-status.service';
import axios from 'axios';
import { apiOrigin } from 'src/app/services/api-origin';

@Component({
    selector: 'app-private',
    templateUrl: './private.component.html',
    styleUrls: ['./private.component.scss'],
    standalone: false
})
export class PrivateComponent implements OnInit {
	@ViewChild('orgDialog', { static: true }) dialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('verifyEmailDialog', { static: true }) verifyEmailDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('orgSearchInput') orgSearchInputEl?: ElementRef<HTMLInputElement>;

	showDropdown = false;
	showOrgDropdown = false;
	showMoreDropdown = false;
	showOverlay = false;
	showAddOrganisationModal = false;
	isEmailVerified = true;
	apiURL = this.generalService.apiURL();
	organisations?: ORGANIZATION_DATA[];
	userOrganization?: ORGANIZATION_DATA;
	convoyVersion: string = '';
	isLoadingOrganisations = false;
	orgPagination?: { has_next_page: boolean; next_page_cursor: string; total?: number };
	orgSearch = '';
	orgSearchEnabled = false;
	// Tracks whether the user belongs to any organisation at all. Unlike
	// `organisations` (which becomes the filtered dropdown list and can be empty
	// during a no-match search), this stays true so the switcher and app shell
	// don't unmount while searching.
	hasOrganisations = false;
	showOrgSearchInput = false;
	isSearchingOrganisations = false;
	loadingMoreOrganisations = false;
	private orgSearchTimeout: any;
	addOrganisationForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required]
	});
	creatingOrganisation = false;
	checkTokenInterval: any;
	isInstanceAdmin = false;
	onboardingSteps = [
		{ step: 'Create an Organization', id: 'organisation', description: 'Add your organization details and get set up.', stepColor: 'bg-[#416FF4] shadow-[0_22px_24px_0px_rgba(65,111,244,0.2)]', class: 'border-[rgba(65,111,244,0.2)]', currentStage: 'current' },
		{
			step: 'Create your first project',
			id: 'project',
			description: 'Set up all the information for creating your webhook events.',
			stepColor: 'bg-[#47B38D] shadow-[0_22px_24px_0px_rgba(43,214,123,0.2)]',
			class: 'border-[rgba(71,179,141,0.36)]',
			currentStage: 'pending'
		}
	];
	private jwtHelper: JwtHelperService = new JwtHelperService();
	private shouldShowOrgSubscription: Subscription | undefined;

	constructor(private generalService: GeneralService, public router: Router, public privateService: PrivateService, private formBuilder: FormBuilder, public licenseService: LicensesService, private rbacService: RbacService, public trialStatusService: TrialStatusService) {}

	async ngOnInit() {
		this.shouldShowOrgModal();

		// Check if user changed and clear cache if needed
		const currentUserId = this.authDetails()?.uid;
		const lastUserId = localStorage.getItem('CONVOY_LAST_USER_ID');
		if (lastUserId && currentUserId && lastUserId !== currentUserId) {
			// User changed, clear cache and localStorage items
			this.privateService.clearCache(true);
		}
		if (currentUserId) {
			localStorage.setItem('CONVOY_LAST_USER_ID', currentUserId);
		}

		this.checkIfTokenIsExpired();
		await Promise.all([this.getConfiguration(), this.getUserDetails(), this.getOrganizations()]);
		await this.licenseService.loadAllLicenses();
		await this.checkInstanceAdminAccess();
		// Fire-and-forget: the trial pill should not block the shell from rendering.
		void this.trialStatusService.refresh();
	}

	ngOnDestroy() {
		if (this.shouldShowOrgSubscription) {
			this.shouldShowOrgSubscription.unsubscribe();
			this.shouldShowOrgSubscription = undefined;
		}
		if (this.checkTokenInterval) {
			clearTimeout(this.checkTokenInterval);
			this.checkTokenInterval = undefined;
		}
	}

	async logout() {
		// Clear intervals and subscriptions first to prevent any ongoing operations
		if (this.checkTokenInterval) {
			clearTimeout(this.checkTokenInterval);
			this.checkTokenInterval = undefined;
		}
		if (this.shouldShowOrgSubscription) {
			this.shouldShowOrgSubscription.unsubscribe();
			this.shouldShowOrgSubscription = undefined;
		}

		let token: string | null = null;
		let userId: string | null = null;
		try {
			const authData = localStorage.getItem('CONVOY_AUTH');
			const auth = authData ? JSON.parse(authData) : null;
			token = auth?.access_token ?? null;
			userId = auth?.uid ?? null;
		} catch (_) {
			// Continue logout even when localStorage auth payload is malformed.
			token = null;
			userId = null;
		}

		// Revoke queue iframe session while dashboard auth token is still valid.
		try {
			if (token) {
				const apiBase = apiOrigin();
				await axios.delete(`${apiBase}/queue/monitoring/session`, {
					headers: { Authorization: `Bearer ${token}`, 'X-Convoy-Version': '2024-04-01' },
					withCredentials: true
				});
			}
		} catch (_) {}

		try {
			await Promise.race([
				this.privateService.logout(),
				new Promise((_, reject) => setTimeout(() => reject(new Error('Logout timeout')), 5000))
			]);
		} catch (error) {
			// Error handled silently - cleanup continues
		}

		this.privateService.clearCache();
		this.licenseService.clearLicenses();
		this.trialStatusService.clear();

		localStorage.removeItem('CONVOY_AUTH');
		localStorage.removeItem('CONVOY_AUTH_TOKENS');
		localStorage.removeItem('CONVOY_LAST_USER_ID');
		localStorage.removeItem('CONVOY_PORTAL_LINK_AUTH_TOKEN');
		localStorage.removeItem('GOOGLE_OAUTH_ID_TOKEN');
		localStorage.removeItem('GOOGLE_OAUTH_USER_INFO');
		localStorage.removeItem('AUTH_TYPE');
		if (userId) {
			localStorage.removeItem(`CONVOY_LAST_USER_ROLE_${userId}`);
		}
		localStorage.removeItem('CONVOY_LAST_USER_ROLE');

		this.router.navigateByUrl('/login');
	}

	authDetails() {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		return authDetails ? JSON.parse(authDetails) : false;
	}

	shouldMountAppRouter(): boolean {
		return !this.isLoadingOrganisations && (this.hasOrganisations || this.router.url.startsWith('/user-settings'));
	}

	async getConfiguration() {
		try {
			const response = await this.privateService.getConfiguration();
			this.convoyVersion = response.data[0].api_version;
		} catch {}
	}

	async getOrganizations(refresh: boolean = false) {
		this.isLoadingOrganisations = true;
		try {
			const response = await this.privateService.getOrganizations({ refresh });
			this.organisations = response.data.content;
			this.hasOrganisations = !!this.organisations?.length;
			this.orgPagination = response.data.pagination;
			// Show the search affordance once the user has more orgs than a single page.
			if (this.orgPagination?.has_next_page) this.orgSearchEnabled = true;
			this.isLoadingOrganisations = false;
			if (this.organisations?.length) await this.checkForSelectedOrganisation();
			return;
		} catch (error) {
			this.isLoadingOrganisations = false;
			return error;
		}
	}

	toggleOrgSearch() {
		this.showOrgSearchInput = !this.showOrgSearchInput;
		if (this.showOrgSearchInput) {
			setTimeout(() => this.orgSearchInputEl?.nativeElement.focus(), 0);
			return;
		}
		// Collapsing the search resets back to the unfiltered first page.
		if (this.orgSearch) {
			this.orgSearch = '';
			this.runOrgSearch();
		}
	}

	onOrgSearch(term: string) {
		this.orgSearch = term;
		clearTimeout(this.orgSearchTimeout);
		this.orgSearchTimeout = setTimeout(() => this.runOrgSearch(), 300);
	}

	async runOrgSearch() {
		this.isSearchingOrganisations = true;
		try {
			const response = await this.privateService.getOrganizations({ q: this.orgSearch || undefined, refresh: true });
			this.organisations = response.data.content;
			this.orgPagination = response.data.pagination;
		} catch (error) {
		} finally {
			this.isSearchingOrganisations = false;
		}
	}

	async loadMoreOrganisations() {
		if (!this.orgPagination?.has_next_page || this.loadingMoreOrganisations) return;
		this.loadingMoreOrganisations = true;
		try {
			const response = await this.privateService.getOrganizations({
				q: this.orgSearch || undefined,
				next_page_cursor: this.orgPagination.next_page_cursor,
				refresh: true
			});
			this.organisations = [...(this.organisations || []), ...response.data.content];
			this.orgPagination = response.data.pagination;
		} catch (error) {
		} finally {
			this.loadingMoreOrganisations = false;
		}
	}

	async getUserDetails() {
		const auth = this.authDetails();
		if (!auth || typeof auth !== 'object' || !auth.uid) return;

		try {
			const response = await this.privateService.getUserDetails({ userId: auth.uid });
			const userDetails = response.data;
			this.isEmailVerified = userDetails?.email_verified;
		} catch (error) {}
	}

	async selectOrganisation(organisation: ORGANIZATION_DATA) {
		this.isLoadingOrganisations = true;
		this.privateService.organisationDetails = organisation;
		this.userOrganization = organisation;

		const userId = this.authDetails()?.uid;
		if (userId) {
			this.privateService.setUserOrg(userId, organisation);
		} else {
			localStorage.setItem('CONVOY_ORG', JSON.stringify(organisation));
		}

		await this.licenseService.setLicenses();
		await this.privateService.getProjects({ refresh: true });
		await this.checkInstanceAdminAccess();
		// Trial state is per-org; refresh the pill for the newly selected org.
		void this.trialStatusService.refresh();
		this.showOrgDropdown = false;

		try {
			await this.router.navigateByUrl('/projects');
		} finally {
			this.isLoadingOrganisations = false;
		}
	}

	async checkForSelectedOrganisation() {
		if (!this.organisations?.length) return;

		const selectedOrganisation = localStorage.getItem('CONVOY_ORG');
		if (!selectedOrganisation || selectedOrganisation === 'undefined') {
			await this.updateOrganisationDetails();
			return;
		}

		const organisationDetails = JSON.parse(selectedOrganisation);
		// Trust the stored org by uid. With paginated/searched lists the selected org
		// may not be in the currently loaded page, so we no longer reset to the first org.
		if (organisationDetails?.uid) {
			this.privateService.organisationDetails = organisationDetails;
			this.userOrganization = organisationDetails;
			await this.checkInstanceAdminAccess();
		} else {
			await this.updateOrganisationDetails();
		}
	}

	async updateOrganisationDetails() {
		if (!this.organisations?.length) return;

		this.privateService.organisationDetails = this.organisations[0];
		this.userOrganization = this.organisations[0];

		// Save to per-user storage
		const userId = this.authDetails()?.uid;
		if (userId) {
			this.privateService.setUserOrg(userId, this.organisations[0]);
		} else {
			localStorage.setItem('CONVOY_ORG', JSON.stringify(this.organisations[0]));
		}

		await this.checkInstanceAdminAccess();
	}

	get showHelpCard() {
		const formUrls = ['apps/new', 'sources/new', 'subscriptions/new'];
		const checkForCreateForms = formUrls.some(url => this.router.url.includes(url));
		return this.router.url === '/projects/new' || checkForCreateForms;
	}

	async addNewOrganisation() {
		if (this.addOrganisationForm.invalid) {
			(<any>this.addOrganisationForm).values(this.addOrganisationForm.controls).forEach((control: FormControl) => {
				control?.markAsTouched();
			});
			return;
		}
		this.creatingOrganisation = true;

		try {
			const response = await this.privateService.addOrganisation(this.addOrganisationForm.value);

			this.generalService.showNotification({ style: 'success', message: response.message });
			this.creatingOrganisation = false;
			this.dialog.nativeElement.close();

			await this.getOrganizations(true);
			await this.selectOrganisation(response.data);
		} catch {
			this.creatingOrganisation = false;
		}
	}

	async getRefreshToken() {
		try {
			await this.privateService.getRefreshToken();
			clearTimeout(this.checkTokenInterval);
			this.checkIfTokenIsExpired();
		} catch (error) {
			clearTimeout(this.checkTokenInterval);
		}
	}

	async checkIfTokenIsExpired() {
		let authTokens = localStorage.CONVOY_AUTH_TOKENS;
		authTokens = authTokens ? JSON.parse(authTokens) : false;

		if (!authTokens) return;

		const currentTime = new Date();
		const tokenExpiryTime = this.jwtHelper.getTokenExpirationDate(authTokens.access_token);

		if (tokenExpiryTime) {
			const expiryPeriodInSeconds = differenceInSeconds(tokenExpiryTime, currentTime);
			if (expiryPeriodInSeconds <= 600) return this.getRefreshToken();

			this.inTimeoutCheck(expiryPeriodInSeconds - 600);
		}
	}

	shouldShowOrgModal() {
		this.shouldShowOrgSubscription = this.privateService.showOrgModal.subscribe(
			(val: boolean) => {
				if (val) this.dialog?.nativeElement?.showModal();
			},
			error => {
				return error;
			}
		);
	}

	inTimeoutCheck(time: number) {
		this.checkTokenInterval = setTimeout(() => {
			this.checkIfTokenIsExpired();
		}, time * 1000 + 1000);
	}

	async checkInstanceAdminAccess() {
		try {
			const userRole = await this.rbacService.getUserRole();
			this.isInstanceAdmin = userRole === 'INSTANCE_ADMIN';
		} catch (error) {
			this.isInstanceAdmin = false;
		}
	}

	getOrgLimitMessage(): string {
		if (!this.licenseService.hasLicense('org_limit')) {
			if (!this.licenseService.isLimitAvailable('org_limit')) {
				return 'Business';
			}
			if (this.licenseService.isLimitAvailable('org_limit') && this.licenseService.isLimitReached('org_limit')) {
				const limitInfo = this.licenseService.getLimitInfo('org_limit');
				const current = limitInfo?.current ?? 0;
				const limit = limitInfo?.limit === -1 ? '∞' : (limitInfo?.limit ?? 0);
				return `Limit reached (${current}/${limit})`;
			}
		}
		return '';
	}
}
