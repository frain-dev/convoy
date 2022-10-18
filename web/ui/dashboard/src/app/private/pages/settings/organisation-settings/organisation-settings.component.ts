import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { SettingsService } from '../settings.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { Router } from '@angular/router';

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
	editOrganisationForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required]
	});
	constructor(private formBuilder: FormBuilder, private settingService: SettingsService, private generalService: GeneralService, private router:Router) {}

	ngOnInit() {
        this.getOrganisationDetails()
    }

	async updateOrganisation() {
		if (this.editOrganisationForm.invalid) return this.editOrganisationForm.markAllAsTouched();
		this.isEditingOrganisation = true;
		try {
			const response = await this.settingService.updateOrganisation({ org_id: this.organisationId, body: this.editOrganisationForm.value });
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
