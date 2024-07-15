import {Component, OnInit} from '@angular/core';
import {FormBuilder, FormGroup, Validators} from '@angular/forms';
import {SettingsService} from '../settings.service';
import {GeneralService} from 'src/app/services/general/general.service';

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
		retention_policy_enabled: [true],
		retention_policy: this.formBuilder.group({
			policy: [720]
		}),
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

	configurations = [
		{ uid: 'retention_policy', name: 'Retention Policy', show: false },
		{ uid: 'storage_policy', name: 'Storage Policy', show: false }
	];

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
			this.configForm.get('retention_policy.policy')?.patchValue(this.getHours(configurations.retention_policy.policy));

			this.isFetchingConfig = false;
		} catch {
			this.isFetchingConfig = false;
		}
	}

	async updateConfigSettings() {
		if (this.configForm.value.storage_policy.type === 'on_prem') delete this.configForm.value.storage_policy.s3;
		if (this.configForm.value.storage_policy.type === 's3') delete this.configForm.value.storage_policy.on_prem;
		if (typeof this.configForm.value.retention_policy.policy === 'number') this.configForm.value.retention_policy.policy = `${this.configForm.value.retention_policy.policy}h`;

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

	toggleConfigForm(configValue: string) {
		this.configurations.forEach(config => {
			if (config.uid === configValue) config.show = !config.show;
			if (configValue === 'retention_policy' && config.uid === 'retention_policy') this.configForm.patchValue({ retention_policy_enabled: config.show });
		});
	}

	showConfig(configValue: string): boolean {
		return this.configurations.find(config => config.uid === configValue)?.show || false;
	}

	getHours(hours: any) {
		const [digits, _] = hours.match(/\D+|\d+/g);
		return parseInt(digits);
	}
}
