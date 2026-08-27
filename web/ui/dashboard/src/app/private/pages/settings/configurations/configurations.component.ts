import {Component, OnInit} from '@angular/core';
import {FormBuilder, FormGroup, Validators} from '@angular/forms';
import {SettingsService} from '../settings.service';
import {GeneralService} from 'src/app/services/general/general.service';

@Component({
    selector: 'configurations',
    templateUrl: './configurations.component.html',
    styleUrls: ['./configurations.component.scss'],
    standalone: false
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
	configLoaded = false;
	loaderIndex: number[] = [0, 1, 2];
	// Storage secrets and on-prem path are optional on update: GET redacts them,
	// and blank on PUT means keep (preserveStoragePolicySecrets).
	configForm: FormGroup = this.formBuilder.group({
		is_analytics_enabled: [null, Validators.required],
		is_signup_enabled: [null, Validators.required],
		webhook_archiving: this.formBuilder.group({
			enabled: [false]
		}),
		retention_policy: this.formBuilder.group({
			enabled: [true],
			period: [720]
		}),
		storage_policy: this.formBuilder.group({
			type: [null, Validators.required],
			on_prem: this.formBuilder.group({
				path: [null]
			}),
			s3: this.formBuilder.group({
				bucket: [null, Validators.required],
				region: [null, Validators.required],
				access_key: [null],
				secret_key: [null],
				session_token: [null]
			})
		})
	});

	configurations = [
		{ uid: 'retention_policy', name: 'Retention Period', show: false },
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
			const period = configurations.retention_policy?.period || configurations.retention_policy?.policy;
			if (period) {
				this.configForm.get('retention_policy.period')?.patchValue(this.getHours(period));
				this.configurations.forEach(c => {
					if (c.uid === 'retention_policy') c.show = true;
				});
			}
			if (configurations.storage_policy?.type) {
				this.configurations.forEach(c => {
					if (c.uid === 'storage_policy') c.show = true;
				});
			}

			this.configLoaded = true;
			this.isFetchingConfig = false;
		} catch {
			this.configLoaded = false;
			this.isFetchingConfig = false;
		}
	}

	async updateConfigSettings() {
		if (!this.configLoaded) {
			return;
		}
		const payload = structuredClone(this.configForm.value);
		// Omit storage_policy when type is unset or not editable in this form
		// (azure_blob has no fields yet). ValidateForUpdate + preserve keep
		// stored Azure when type is resent without a nested object; skipping
		// avoids accidental type flips via the on_prem/s3 radios.
		if (payload.storage_policy?.type !== 'on_prem' && payload.storage_policy?.type !== 's3') {
			delete payload.storage_policy;
		} else if (payload.storage_policy.type === 'on_prem') {
			delete payload.storage_policy.s3;
		} else if (payload.storage_policy.type === 's3') {
			delete payload.storage_policy.on_prem;
		}
		if (typeof payload.retention_policy?.period === 'number') {
			payload.retention_policy.period = `${payload.retention_policy.period}h`;
		}

		this.isUpdatingConfig = true;
		try {
			const response = await this.settingService.updateConfigSettings(payload);
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
