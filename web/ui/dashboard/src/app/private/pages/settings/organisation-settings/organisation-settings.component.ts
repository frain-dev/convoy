import {Component, inject, OnInit} from '@angular/core';
import {FormBuilder, FormGroup, Validators} from '@angular/forms';
import {SettingsService} from '../settings.service';
import {GeneralService} from 'src/app/services/general/general.service';
import {Router} from '@angular/router';
import {PrivateService} from 'src/app/private/private.service';
import {RbacService} from 'src/app/services/rbac/rbac.service';
import {LicensesService} from 'src/app/services/licenses/licenses.service';

@Component({
	selector: 'organisation-settings',
	templateUrl: './organisation-settings.component.html',
	styleUrls: ['./organisation-settings.component.scss']
})
export class OrganisationSettingsComponent implements OnInit {
	organisationId!: string;
	organisationName!: string;
	showDeleteModal = false;
	isEditingOrganisation = false;
	isDeletingOrganisation = false;
	configuringSSO = false;
	editOrganisationForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required]
	});
	private rbacService = inject(RbacService);

	constructor(
		private formBuilder: FormBuilder,
		private settingService: SettingsService,
		private generalService: GeneralService,
		private router: Router,
		private privateService: PrivateService,
		public licenseService: LicensesService
	) {}

	async ngOnInit() {
		this.getOrganisationDetails();
		if (!(await this.rbacService.userCanAccess('Organisations|MANAGE'))) this.editOrganisationForm.disable();
	}

	async updateOrganisation() {
		if (this.editOrganisationForm.invalid) return this.editOrganisationForm.markAllAsTouched();
		this.isEditingOrganisation = true;
		try {
			const response = await this.settingService.updateOrganisation({ org_id: this.organisationId, body: this.editOrganisationForm.value });
			this.privateService.getOrganizations({ refresh: true });
			this.generalService.showNotification({ style: 'success', message: response.message });
			this.isEditingOrganisation = false;
		} catch {
			this.isEditingOrganisation = false;
		}
	}

	get currentOrg() {
		return this.privateService.getOrganisation;
	}

	getOrganisationDetails() {
		const org = localStorage.getItem('CONVOY_ORG');
		if (org) {
			const organisationDetails = JSON.parse(org);
			this.organisationId = organisationDetails.uid;
			this.organisationName = organisationDetails.name;
			this.editOrganisationForm.patchValue({
				name: organisationDetails.name
			});
		}
	}

	async configureSSO() {
		this.configuringSSO = true;
		try {
			const returnUrl = window.location.href || (window.location.origin + '/');
			const response = await this.settingService.getSSOAdminPortal(returnUrl);
			const portalUrl = response?.data?.portal_url;
			if (portalUrl) {
				window.location.href = portalUrl;
			} else {
				this.generalService.showNotification({ style: 'error', message: 'Invalid response from SSO service' });
				this.configuringSSO = false;
			}
		} catch (err: any) {
			const message = typeof err === 'string' ? err : err?.response?.data?.message || err?.message || 'Failed to open SSO admin portal';
			this.generalService.showNotification({ style: 'error', message });
			this.configuringSSO = false;
		}
	}

	async deleteOrganisation() {
		this.isDeletingOrganisation = true;
		try {
			const response = await this.settingService.deleteOrganisation({ org_id: this.organisationId });
			this.generalService.showNotification({ style: 'success', message: response.message });
			localStorage.removeItem('CONVOY_ORG');
			this.router.navigateByUrl('/').then(() => {
				window.location.reload();
			});
			this.isDeletingOrganisation = false;
		} catch {
			this.isDeletingOrganisation = false;
		}
	}
}
