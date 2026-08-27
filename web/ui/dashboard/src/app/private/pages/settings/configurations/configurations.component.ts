import {Component, HostListener, OnInit} from '@angular/core';
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
	// Last value returned from GET/Save. Used to toast when ownership flips.
	private savedAdminManaged = false;
	// Storage secrets and on-prem path are optional on update: GET redacts them,
	// and blank on PUT means keep (preserveStoragePolicySecrets).
	configForm: FormGroup = this.formBuilder.group({
		admin_managed: [false, Validators.required],
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
				endpoint: [null],
				prefix: [null],
				access_key: [null],
				secret_key: [null],
				session_token: [null]
			}),
			azure_blob: this.formBuilder.group({
				account_name: [null, Validators.required],
				account_key: [null],
				container_name: [null, Validators.required],
				endpoint: [null],
				prefix: [null]
			})
		})
	});

	configurations = [{ uid: 'storage_policy', name: 'Storage Policy', show: false }];

	constructor(private formBuilder: FormBuilder, private settingService: SettingsService, private generalService: GeneralService) {}

	ngOnInit() {
		this.configForm.get('retention_policy.enabled')?.valueChanges.subscribe(enabled => {
			this.syncRetentionPeriodEnabled(!!enabled);
		});
		this.configForm.get('admin_managed')?.valueChanges.subscribe(enabled => {
			this.syncAdminManaged(!!enabled);
		});
		this.fetchConfigSettings();
	}

	get canSave(): boolean {
		return this.configLoaded && this.configForm.dirty && !this.isUpdatingConfig && !this.isFetchingConfig;
	}

	get hasUnsavedChanges(): boolean {
		// Exclude in-flight save/refetch: after PUT the form is still dirty until
		// markAsPristine, and a refetch patch briefly dirties it again.
		return this.configLoaded && this.configForm.dirty && !this.isUpdatingConfig && !this.isFetchingConfig;
	}

	// Reload / tab close while the form is dirty. Sidebar leave is handled by Admin.
	@HostListener('window:beforeunload', ['$event'])
	onBeforeUnload(event: BeforeUnloadEvent) {
		if (!this.hasUnsavedChanges) {
			return;
		}
		event.preventDefault();
		event.returnValue = true;
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
			}
			if (configurations.storage_policy?.type) {
				this.configurations.forEach(c => {
					if (c.uid === 'storage_policy') c.show = true;
				});
			}
			this.syncAdminManaged(!!this.configForm.get('admin_managed')?.value);
			this.syncRetentionPeriodEnabled(!!this.configForm.get('retention_policy.enabled')?.value);
			this.savedAdminManaged = !!this.configForm.get('admin_managed')?.value;

			this.configForm.markAsPristine();
			this.configLoaded = true;
			this.isFetchingConfig = false;
		} catch {
			// Leave configLoaded alone. A failed refetch after save must not clear
			// it: the form still holds editable values, and wiping the flag disables
			// Save and hides dirty leave warnings for later edits.
			this.isFetchingConfig = false;
		}
	}

	async updateConfigSettings() {
		if (!this.canSave) {
			return;
		}
		// getRawValue keeps retention period when Retention is off (control disabled).
		const payload = structuredClone(this.configForm.getRawValue());
		const storageType = payload.storage_policy?.type;
		if (storageType !== 'on_prem' && storageType !== 's3' && storageType !== 'azure_blob') {
			delete payload.storage_policy;
		} else if (storageType === 'on_prem') {
			delete payload.storage_policy.s3;
			delete payload.storage_policy.azure_blob;
		} else if (storageType === 's3') {
			delete payload.storage_policy.on_prem;
			delete payload.storage_policy.azure_blob;
		} else if (storageType === 'azure_blob') {
			delete payload.storage_policy.on_prem;
			delete payload.storage_policy.s3;
		}
		if (typeof payload.retention_policy?.period === 'number') {
			payload.retention_policy.period = `${payload.retention_policy.period}h`;
		}

		const nextAdminManaged = !!payload.admin_managed;
		const ownershipChanged = this.savedAdminManaged !== nextAdminManaged;

		this.isUpdatingConfig = true;
		try {
			const response = await this.settingService.updateConfigSettings(payload);
			this.generalService.showNotification({
				message: ownershipChanged
					? 'Saved. Restart server (and agent) for boot ownership and circuit-breaker defaults.'
					: response.message,
				style: ownershipChanged ? 'info' : 'success'
			});
			// Saved bytes are on the server; clear dirty before refetch so leave
			// confirms and the banner do not treat the post-PUT window as unsaved.
			this.savedAdminManaged = nextAdminManaged;
			this.configForm.markAsPristine();
			this.isUpdatingConfig = false;
			await this.fetchConfigSettings();
		} catch {
			this.isUpdatingConfig = false;
		}
	}

	toggleConfigForm(configValue: string) {
		if (!this.configForm.get('admin_managed')?.value) {
			return;
		}
		this.configurations.forEach(config => {
			if (config.uid === configValue) config.show = !config.show;
		});
	}

	showConfig(configValue: string): boolean {
		return this.configurations.find(config => config.uid === configValue)?.show || false;
	}

	syncAdminManaged(enabled: boolean) {
		for (const name of ['is_analytics_enabled', 'is_signup_enabled', 'retention_policy', 'webhook_archiving', 'storage_policy']) {
			const control = this.configForm.get(name);
			enabled ? control?.enable({ emitEvent: false }) : control?.disable({ emitEvent: false });
		}
		if (enabled) {
			this.syncRetentionPeriodEnabled(!!this.configForm.get('retention_policy.enabled')?.value);
		}
	}

	syncRetentionPeriodEnabled(enabled: boolean) {
		const period = this.configForm.get('retention_policy.period');
		if (!period) {
			return;
		}
		if (enabled && this.configForm.get('admin_managed')?.value) {
			period.enable({ emitEvent: false });
		} else {
			period.disable({ emitEvent: false });
		}
	}

	getHours(hours: any) {
		const [digits, _] = hours.match(/\D+|\d+/g);
		return parseInt(digits);
	}
}
