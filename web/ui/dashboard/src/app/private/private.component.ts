import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { Router } from '@angular/router';
import { ORGANIZATION_DATA } from '../models/organisation.model';
import { GeneralService } from '../services/general/general.service';
import { PrivateService } from './private.service';
import { FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
import { JwtHelperService } from '@auth0/angular-jwt';
import { differenceInSeconds } from 'date-fns';

@Component({
	selector: 'app-private',
	templateUrl: './private.component.html',
	styleUrls: ['./private.component.scss']
})
export class PrivateComponent implements OnInit {
	@ViewChild('orgDialog', { static: true }) dialog!: ElementRef<HTMLDialogElement>;

	showDropdown = false;
	showOrgDropdown = false;
	showMoreDropdown = false;
	showOverlay = false;
	showAddOrganisationModal = false;
	showVerifyEmailModal = false;
	isEmailVerified = true;
	apiURL = this.generalService.apiURL();
	organisations?: ORGANIZATION_DATA[];
	userOrganization?: ORGANIZATION_DATA;
	convoyVersion: string = '';
	isLoadingOrganisations = false;
	showCreateOrganisationModal = this.privateService.showCreateOrgModal;
	addOrganisationForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required]
	});
	creatingOrganisation = false;
	checkTokenInterval: any;
	private jwtHelper: JwtHelperService = new JwtHelperService();

	constructor(private generalService: GeneralService, private router: Router, public privateService: PrivateService, private formBuilder: FormBuilder) {}

	async ngOnInit() {
		this.checkIfTokenIsExpired();
		await Promise.all([this.getConfiguration(), this.getUserDetails(), this.getOrganizations()]);
	}

	async logout() {
		await this.privateService.logout();
		localStorage.removeItem('CONVOY_AUTH');
		this.router.navigateByUrl('/login');
	}

	authDetails() {
		const authDetails = localStorage.getItem('CONVOY_AUTH');
		return authDetails ? JSON.parse(authDetails) : false;
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
			if (this.organisations?.length === 0) return this.router.navigateByUrl('/get-started');
			this.checkForSelectedOrganisation();
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
		localStorage.setItem('CONVOY_ORG', JSON.stringify(organisation));
		this.showOrgDropdown = false;
		this.privateService.getProjectsHelper({ refresh: true });
	}

	checkForSelectedOrganisation() {
		if (!this.organisations?.length) return;

		const selectedOrganisation = localStorage.getItem('CONVOY_ORG');
		if (!selectedOrganisation || selectedOrganisation === 'undefined') return this.updateOrganisationDetails();

		const organisationDetails = JSON.parse(selectedOrganisation);
		if (this.organisations.find(org => org.uid === organisationDetails.uid)) {
			this.privateService.organisationDetails = organisationDetails;
			this.userOrganization = organisationDetails;
		} else {
			this.updateOrganisationDetails();
		}
	}

	updateOrganisationDetails() {
		if (!this.organisations?.length) return;

		this.privateService.organisationDetails = this.organisations[0];
		this.userOrganization = this.organisations[0];
		localStorage.setItem('CONVOY_ORG', JSON.stringify(this.organisations[0]));
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
			this.showCreateOrganisationModal = false;
			this.privateService.showCreateOrgModal = false;
			this.dialog.nativeElement.close();

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

	inTimeoutCheck(time: number) {
		this.checkTokenInterval = setTimeout(() => {
			this.checkIfTokenIsExpired();
		}, time * 1000 + 1000);
	}
}
