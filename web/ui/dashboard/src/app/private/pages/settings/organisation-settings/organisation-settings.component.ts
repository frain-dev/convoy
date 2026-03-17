import {Component, ElementRef, inject, OnInit, ViewChild} from '@angular/core';
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
	currentWorkspaceSlug: string | null = null;
	workspaceSlugInput = '';
	workspaceSlugError = '';
	isSavingSlug = false;
	isEditingOrganisation = false;
	isDeletingOrganisation = false;
	configuringSSO = false;
	/** True when this org's license has enterprise_sso; false or null when not or unknown. */
	orgHasEnterpriseSSO: boolean | null = null;
	editOrganisationForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required]
	});
	@ViewChild('slugDialog') slugDialog!: ElementRef<HTMLDialogElement>;
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
			this.currentWorkspaceSlug = typeof organisationDetails.slug === 'string' && organisationDetails.slug.length > 0 ? organisationDetails.slug : null;
			this.loadWorkspaceSlug();
			this.loadOrgLicenseForSSO();
		}
	}

	private async loadWorkspaceSlug() {
		if (!this.organisationId) return;
		try {
			const response = await this.settingService.getOrganisation({ org_id: this.organisationId });
			const slug = response?.data?.slug;
			this.currentWorkspaceSlug = typeof slug === 'string' && slug.length > 0 ? slug : null;
		} catch {
			// Keep existing value from local storage on fetch failure.
		}
	}

	/** Load this org's license so Configure SSO visibility uses org license, not instance. */
	private loadOrgLicenseForSSO() {
		if (!this.organisationId) return;
		this.licenseService.hasEnterpriseSSO(this.organisationId).then((has) => (this.orgHasEnterpriseSSO = has));
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

	startEditingSlug(): void {
		this.workspaceSlugInput = this.currentWorkspaceSlug ?? '';
		this.workspaceSlugError = '';
	}

	cancelEditingSlug(): void {
		this.workspaceSlugInput = '';
		this.workspaceSlugError = '';
	}

	closeSlugDialog(): void {
		this.cancelEditingSlug();
		this.slugDialog?.nativeElement?.close();
	}

	async setWorkspaceSlug(): Promise<void> {
		const slug = this.workspaceSlugInput.trim().toLowerCase();
		if (!slug) {
			this.workspaceSlugError = 'Enter a slug';
			return;
		}
		if (slug.length < 2 || slug.length > 64) {
			this.workspaceSlugError = 'Slug must be 2-64 characters';
			return;
		}
		if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug)) {
			this.workspaceSlugError = 'Use only lowercase letters, numbers, and hyphens';
			return;
		}

		this.workspaceSlugError = '';
		this.isSavingSlug = true;
		try {
			const res = await this.settingService.updateOrganisationSlug({
				org_id: this.organisationId,
				body: { slug }
			});
			const data = res?.data as { slug?: string };
			const newSlug = data?.slug ?? slug;
			this.currentWorkspaceSlug = newSlug;
			this.workspaceSlugInput = '';
			this.closeSlugDialog();
			this.generalService.showNotification({ style: 'success', message: this.currentWorkspaceSlug ? `Workspace slug set to "${this.currentWorkspaceSlug}".` : 'Workspace slug updated.' });
		} catch (err: any) {
			const msg = (typeof err === 'string' ? err : '') || err?.response?.data?.message || err?.message || '';
			const msgLower = String(msg).toLowerCase();
			const isDuplicateSlug = msgLower.includes('slug is already taken') || msgLower.includes('already taken');
			this.workspaceSlugError = isDuplicateSlug ? 'This slug is already taken. Choose another.' : (msg || 'Failed to set slug.');
		} finally {
			this.isSavingSlug = false;
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
		} catch (err: any) {
			const msg = err?.response?.data?.message || err?.message || 'Failed to deactivate organisation.';
			this.generalService.showNotification({ style: 'error', message: msg });
			this.isDeletingOrganisation = false;
		}
	}
}
