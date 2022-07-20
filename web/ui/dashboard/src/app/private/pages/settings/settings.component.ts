import { CommonModule } from '@angular/common';
import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { InputComponent } from 'src/app/components/input/input.component';
import { PageComponent } from 'src/app/components/page/page.component';
import { RadioComponent } from 'src/app/components/radio/radio.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { ToggleComponent } from 'src/app/components/toggle/toggle.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { DeleteModalModule } from '../../components/delete-modal/delete-modal.module';
import { SettingsService } from './settings.service';

@Component({
	selector: 'convoy-settings',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, CardComponent, PageComponent, DeleteModalModule, InputComponent, SelectComponent, RadioComponent, ToggleComponent, ButtonComponent],
	templateUrl: './settings.component.html',
	styleUrls: ['./settings.component.scss']
})
export class SettingsComponent implements OnInit {
	activePage: 'organisation settings' | 'configuration settings' = 'organisation settings';
	settingsMenu: ['organisation settings', 'configuration settings'] = ['organisation settings', 'configuration settings'];
	regions = [
		'us-east-2',
		'us-east-1',
		'us-west-1',
		'us-west-2',
		'af-south-1',
		'ap-east-1',
		'ap-southeast-3',
		'ap-south-1',
		'ap-northeast-3',
		'ap-northeast-2',
		'ap-southeast-1',
		'ap-southeast-2',
		'ap-northeast-1',
		'ca-central-1',
		'cn-north-1',
		'cn-northwest-1',
		'eu-central-1',
		'eu-west-1',
		'eu-west-2',
		'eu-south-1',
		'eu-west-3',
		'eu-north-1',
		'me-south-1',
		'sa-east-1'
	];
	showDeactivateAccountModal = false;
	isEditingOrganisation = false;
	isUpdatingConfig = false;
	isDeletingOrganisation = false;
	showDeleteModal = false;
	organisationId!: string;
	organisationName!: string;
	editOrganisationForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required]
	});
	configForm: FormGroup = this.formBuilder.group({
		is_analytics_enabled: [null, Validators.required],
		storage_policy: this.formBuilder.group({
			type: [null, Validators.required],
			on_prem: this.formBuilder.group({
				path: [null, Validators.required]
			}),
			s3: this.formBuilder.group({
				bucket: [null, Validators.required],
				region: [null, Validators.required],
				access_key: [null, Validators.required],
				secret_key: [null, Validators.required],
				session_token: [null]
			})
		})
	});
	constructor(private settingService: SettingsService, private generalService: GeneralService, private formBuilder: FormBuilder, private router: Router, private route:ActivatedRoute) {}

	ngOnInit() {
		this.toggleActivePage(this.route.snapshot.queryParams?.activePage ?? 'organisation settings');
		this.getOrganisationDetails();
		this.fetchConfigSettings();
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

	async fetchConfigSettings() {
		try {
			const response = await this.settingService.fetchConfigSettings();
			const configurations = response.data[0];
			this.configForm.patchValue(configurations);
		} catch {}
	}

	async updateConfigSettings() {
		if (this.configForm.value.storage_policy.type === 'on_prem') delete this.configForm.value.storage_policy.s3;
		if (this.configForm.value.storage_policy.type === 's3') delete this.configForm.value.storage_policy.on_prem;
		this.isUpdatingConfig = true;
		try {
			const response = await this.settingService.updateConfigSettings(this.configForm.value);
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.isUpdatingConfig = false;
			this.fetchConfigSettings()
		} catch {
			this.isUpdatingConfig = false;
		}
	}

	toggleActivePage(activePage: 'organisation settings' | 'configuration settings') {
		this.activePage = activePage;
		if (!this.router.url.split('/')[2]) this.addPageToUrl();
	}

	addPageToUrl() {
		const queryParams: any = {};
		queryParams.activePage = this.activePage;
		this.router.navigate([], { queryParams: Object.assign({}, queryParams) });
	}
}
