import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { SettingsService } from '../settings.service';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'configurations',
	templateUrl: './configurations.component.html',
	styleUrls: ['./configurations.component.scss']
})
export class ConfigurationsComponent implements OnInit {
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
	isUpdatingConfig = false;
	showDeleteModal = false;
	isFetchingConfig = false;
	loaderIndex: number[] = [0, 1, 2];
	configForm: FormGroup = this.formBuilder.group({
		is_analytics_enabled: [null, Validators.required],
		is_signup_enabled: [null, Validators.required],
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

	constructor(private formBuilder: FormBuilder, private settingService: SettingsService, private generalService: GeneralService) {}

	ngOnInit() {
		this.fetchConfigSettings();
	}

	async fetchConfigSettings() {
		this.isFetchingConfig = true;
		try {
			const response = await this.settingService.fetchConfigSettings();
			const configurations = response.data[0];
			this.configForm.patchValue(configurations);
			this.isFetchingConfig = false;
		} catch {
			this.isFetchingConfig = false;
		}
	}

	async updateConfigSettings() {
		if (this.configForm.value.storage_policy.type === 'on_prem') delete this.configForm.value.storage_policy.s3;
		if (this.configForm.value.storage_policy.type === 's3') delete this.configForm.value.storage_policy.on_prem;
		this.isUpdatingConfig = true;
		try {
			const response = await this.settingService.updateConfigSettings(this.configForm.value);
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.isUpdatingConfig = false;
			this.fetchConfigSettings();
		} catch {
			this.isUpdatingConfig = false;
		}
	}
}
