import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { Router } from '@angular/router';
import { ORGANIZATION_DATA } from '../models/organisation.model';
import { GeneralService } from '../services/general/general.service';
import { PrivateService } from './private.service';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { JwtHelperService } from '@auth0/angular-jwt';
import { differenceInSeconds } from 'date-fns';
import { Observable, Subscription } from 'rxjs';
import { LicensesService } from '../services/licenses/licenses.service';
import { RbacService } from '../services/rbac/rbac.service';

@Component({
	selector: 'app-private',
	templateUrl: './private.component.html',
	styleUrls: ['./private.component.scss']
})
export class PrivateComponent implements OnInit {
	@ViewChild('orgDialog', { static: true }) dialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('verifyEmailDialog', { static: true }) verifyEmailDialog!: ElementRef<HTMLDialogElement>;

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

	constructor(private generalService: GeneralService, public router: Router, public privateService: PrivateService, private formBuilder: FormBuilder, public licenseService: LicensesService, private rbacService: RbacService) {}

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
		await Promise.all([this.getConfiguration(), this.licenseService.setLicenses(), this.getUserDetails(), this.getOrganizations()]);
		// Check instance admin access after organizations are loaded
		await this.checkInstanceAdminAccess();
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

		try {
			await Promise.race([
				this.privateService.logout(),
				new Promise((_, reject) => setTimeout(() => reject(new Error('Logout timeout')), 5000))
			]);
		} catch (error) {
			// Error handled silently - cleanup continues
		}

		this.privateService.clearCache();

		localStorage.removeItem('CONVOY_AUTH');
		localStorage.removeItem('CONVOY_AUTH_TOKENS');
		localStorage.removeItem('CONVOY_LAST_USER_ID');
		localStorage.removeItem('CONVOY_PORTAL_LINK_AUTH_TOKEN');
		localStorage.removeItem('GOOGLE_OAUTH_ID_TOKEN');
		localStorage.removeItem('GOOGLE_OAUTH_USER_INFO');
		localStorage.removeItem('AUTH_TYPE');

		this.router.navigateByUrl('/login');
	}

	authDetails() {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		return authDetails ? JSON.parse(authDetails) : false;
	}

	shouldMountAppRouter(): boolean {
		return !this.isLoadingOrganisations && (Boolean(this.organisations?.length) || this.router.url.startsWith('/user-settings'));
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
			this.isLoadingOrganisations = false;
			if (this.organisations?.length) this.checkForSelectedOrganisation();
			return;
		} catch (error) {
			this.isLoadingOrganisations = false;
			return error;
		}
	}

	async getUserDetails() {
		try {
			const response = await this.privateService.getUserDetails({ userId: this.authDetails()?.uid });
			const userDetails = response.data;
			this.isEmailVerified = userDetails?.email_verified;
		} catch (error) {}
	}

	async selectOrganisation(organisation: ORGANIZATION_DATA) {
		this.isLoadingOrganisations = true;
		this.privateService.organisationDetails = organisation;
		this.userOrganization = organisation;

		// Save to per-user storage
		const userId = this.authDetails()?.uid;
		if (userId) {
			this.privateService.setUserOrg(userId, organisation);
		} else {
			localStorage.setItem('CONVOY_ORG', JSON.stringify(organisation));
		}

		await this.privateService.getProjects({ refresh: true });
		await this.checkInstanceAdminAccess();
		this.showOrgDropdown = false;

		this.router.navigateByUrl('/projects');
		setInterval(() => {
			this.isLoadingOrganisations = false;
		}, 1000);
	}

	async checkForSelectedOrganisation() {
		if (!this.organisations?.length) return;

		const selectedOrganisation = localStorage.getItem('CONVOY_ORG');
		if (!selectedOrganisation || selectedOrganisation === 'undefined') {
			await this.updateOrganisationDetails();
			return;
		}

		const organisationDetails = JSON.parse(selectedOrganisation);
		if (this.organisations.find(org => org.uid === organisationDetails.uid)) {
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
			this.licenseService.setLicenses();

			await this.getOrganizations(true);
			this.selectOrganisation(response.data);
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
}
