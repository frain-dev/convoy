import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { GeneralService } from 'src/app/services/general/general.service';
import { OrganisationService } from './organisation.service';

@Component({
	selector: 'app-organisation',
	templateUrl: './organisation.component.html',
	styleUrls: ['./organisation.component.scss']
})
export class OrganisationComponent implements OnInit {
	activePage: 'general settings' | 'danger zone' = 'general settings';
	showDeactivateAccountModal = false;
	isEditingOrganisation = false;
	isDeletingOrganisation = false;
	showDeleteModal = false;
	organisationId!: string;
	organisationName!: string;
	editOrganisationForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required]
	});

	constructor(private formBuilder: FormBuilder, private router: Router, private organisationService: OrganisationService, private generalService: GeneralService) {}

	ngOnInit() {
		this.getOrganisationDetails();
	}
	async logout() {
		try {
			const response: any = await this.organisationService.logout();
			if (response) {
				this.router.navigateByUrl('/login');
				localStorage.clear();
			}
		} catch (error) {}
	}

	async updateOrganisation() {
		if (this.editOrganisationForm.invalid) return this.editOrganisationForm.markAllAsTouched();
		this.isEditingOrganisation = true;
		try {
			const response = await this.organisationService.updateOrganisation({ org_id: this.organisationId, body: this.editOrganisationForm.value });
			this.generalService.showNotification({ style: 'success', message: response.message });
			localStorage.setItem('CONVOY_ORG', JSON.stringify(response.data));
			window.location.reload();
			this.isEditingOrganisation = false;
		} catch {
			this.isEditingOrganisation = false;
		}
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

	async deleteOrganisation() {
		this.isDeletingOrganisation = true;
		try {
			const response = await this.organisationService.deleteOrganisation({ org_id: this.organisationId });
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
