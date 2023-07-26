import { Component, EventEmitter, Input, OnInit, Output, inject, ViewChild, ElementRef } from '@angular/core';
import { FormGroup, Validators, FormBuilder, FormArray } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { PROJECT, VERSIONS } from 'src/app/models/project.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { PrivateService } from '../../private.service';
import { CreateProjectComponentService } from './create-project-component.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';

@Component({
	selector: 'app-create-project-component',
	templateUrl: './create-project-component.component.html',
	styleUrls: ['./create-project-component.component.scss']
})
export class CreateProjectComponent implements OnInit {
	@ViewChild('disableEndpointsDialog', { static: true }) disableEndpointsDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('metaEventsDialog', { static: true }) metaEventsDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('confirmationDialog', { static: true }) confirmationDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('newSignatureDialog', { static: true }) newSignatureDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('tokenDialog', { static: true }) tokenDialog!: ElementRef<HTMLDialogElement>;

	signatureTableHead: string[] = ['Header', 'Version', 'Hash', 'Encoding'];
	projectForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		config: this.formBuilder.group({
			strategy: this.formBuilder.group({
				duration: [null],
				retry_count: [null],
				type: [null]
			}),
			signature: this.formBuilder.group({
				header: [null],
				versions: this.formBuilder.array([])
			}),
			ratelimit: this.formBuilder.group({
				count: [null],
				duration: [null]
			}),
			retention_policy: this.formBuilder.group({
				policy: ['720h']
			}),
			is_retention_policy_enabled: [true],
			disable_endpoint: [false, Validators.required],
			meta_event: this.formBuilder.group({
				is_enabled: [true, Validators.required],
				type: ['http', Validators.required],
				event_type: [[], Validators.required],
				url: ['', Validators.required],
				secret: [null]
			})
		}),
		type: [null, Validators.required]
	});
	newSignatureForm: FormGroup = this.formBuilder.group({
		encoding: [null],
		hash: [null]
	});
	isCreatingProject = false;
	enableMoreConfig = false;
	confirmRegenerateKey = false;
	regeneratingKey = false;
	showMetaEventPrompt = false;
	apiKey!: string;
	hashAlgorithms = ['SHA256', 'SHA512'];
	retryLogicTypes = [
		{ uid: 'linear', name: 'Linear time retry' },
		{ uid: 'exponential', name: 'Exponential time backoff' }
	];
	encodings = ['base64', 'hex'];
	@Output('onAction') onAction = new EventEmitter<any>();
	@Input('action') action: 'create' | 'update' = 'create';
	projectDetails?: PROJECT;
	signatureVersions!: { date: string; content: VERSIONS[] }[];
	configurations = [
		{ uid: 'retry-config', name: 'Retry Config', show: false },
		{ uid: 'rate-limit', name: 'Rate Limit', show: false },
		{ uid: 'retention', name: 'Retention Policy', show: false },
		{ uid: 'signature', name: 'Signature Format', show: false }
	];
	public rbacService = inject(RbacService);
	disableEndpointsModal = false;
	tabs: string[] = ['project config', 'signature history', 'endpoints config', 'meta events'];
	activeTab = 'project config';
	events = ['endpoint.created', 'endpoint.deleted', 'endpoint.updated', 'eventdelivery.success', 'eventdelivery.failed'];

	constructor(private formBuilder: FormBuilder, private createProjectService: CreateProjectComponentService, private generalService: GeneralService, private privateService: PrivateService, public router: Router, private route: ActivatedRoute) {}

	async ngOnInit() {
		if (this.action === 'update') this.getProjectDetails();
		if (!(await this.rbacService.userCanAccess('Project Settings|MANAGE'))) this.projectForm.disable();
		if (this.action === 'update') this.switchTab(this.route.snapshot.queryParams?.activePage ?? 'project config');
	}

	get versions(): FormArray {
		return this.projectForm.get('config.signature.versions') as FormArray;
	}

	get versionsLength(): any {
		const versionsControl = this.projectForm.get('config.signature.versions') as FormArray;
		return versionsControl.length;
	}
	newVersion(): FormGroup {
		return this.formBuilder.group({
			encoding: ['', Validators.required],
			hash: ['', Validators.required]
		});
	}

	addVersion() {
		this.versions.push(this.newVersion());
	}

	toggleConfigForm(configValue: string) {
		this.configurations.forEach(config => {
			if (config.uid === configValue) config.show = !config.show;
		});
	}

	showConfig(configValue: string): boolean {
		return this.configurations.find(config => config.uid === configValue)?.show || false;
	}

	async getProjectDetails() {
		this.enableMoreConfig = true;

		try {
			const projectDetails = this.privateService.getProjectDetails;

			this.projectDetails = projectDetails;

			if (projectDetails?.type === 'incoming') this.tabs = this.tabs.filter(tab => tab !== 'signature history');

			this.projectForm.patchValue(projectDetails);
			this.projectForm.get('config.strategy')?.patchValue(projectDetails.config.strategy);
			this.projectForm.get('config.signature')?.patchValue(projectDetails.config.signature);
			this.projectForm.get('config.ratelimit')?.patchValue(projectDetails.config.ratelimit);

			this.configurations.forEach(config => {
				if (projectDetails?.type === 'outgoing') this.toggleConfigForm(config.uid);
				else if (config.uid !== 'signature') this.toggleConfigForm(config.uid);
			});

			const versions = projectDetails.config.signature.versions;
			if (!versions?.length) return;
			this.signatureVersions = this.generalService.setContentDisplayed(versions);
			versions.forEach((version: { encoding: any; hash: any }, index: number) => {
				this.addVersion();
				this.versions.at(index)?.patchValue({
					encoding: version.encoding,
					hash: version.hash
				});
			});
		} catch {}
	}

	async createProject() {
		const projectFormModal = document.getElementById('projectForm');

		if (this.enableMoreConfig) {
			if (this.newSignatureForm.invalid || this.projectForm.invalid) {
				this.newSignatureForm.markAllAsTouched();
				this.projectForm.markAllAsTouched();
				projectFormModal?.scroll({ top: 0 });
				return;
			}

			this.versions.at(0).patchValue(this.newSignatureForm.value);
		}

		if (!this.enableMoreConfig && this.projectForm.get('name')?.invalid && this.projectForm.get('type')?.invalid) {
			projectFormModal?.scroll({ top: 0 });
			return this.projectForm.markAllAsTouched();
		}
		const dataForNoConfig = this.projectForm.value;
		if (!this.enableMoreConfig) delete dataForNoConfig.config;

		this.isCreatingProject = true;

		try {
			// this createProject service also updates project as active project in localstorage
			const response = await this.createProjectService.createProject(this.enableMoreConfig ? this.projectForm.value : dataForNoConfig);
			await this.privateService.getProjectStat({ refresh: true });

			this.privateService.getProjects({ refresh: true });

			projectFormModal?.scroll({ top: 0, behavior: 'smooth' });
			this.isCreatingProject = false;
			this.projectForm.reset();
			this.apiKey = response.data.api_key.key;
			this.projectDetails = response.data.project;
			if (projectFormModal) projectFormModal.style.overflowY = 'hidden';
			this.tokenDialog.nativeElement.showModal();
		} catch (error) {
			this.isCreatingProject = false;
		}
	}

	async updateProject() {
		this.checkMetaEventsConfig();
		if (this.projectForm.invalid) return this.projectForm.markAllAsTouched();
		if (typeof this.projectForm.value.config.ratelimit.duration === 'string') this.projectForm.value.config.ratelimit.duration = this.getTimeValue(this.projectForm.value.config.ratelimit.duration);
		if (typeof this.projectForm.value.config.strategy.duration === 'string') this.projectForm.value.config.strategy.duration = this.getTimeValue(this.projectForm.value.config.strategy.duration);
		if (typeof this.projectForm.value.config.strategy.retry_count === 'string') this.projectForm.value.config.strategy.retry_count = parseInt(this.projectForm.value.config.strategy.retry_count);
		if (typeof this.projectForm.value.config.ratelimit.count === 'string') this.projectForm.value.config.ratelimit.count = parseInt(this.projectForm.value.config.ratelimit.count);
		this.isCreatingProject = true;

		try {
			// this updateProject service also updates project in localstorage
			const response = await this.createProjectService.updateProject(this.projectForm.value);

			this.generalService.showNotification({ message: 'Project updated successfully!', style: 'success' });
			this.onAction.emit(response.data);
			this.isCreatingProject = false;
		} catch (error) {
			this.isCreatingProject = false;
		}
	}

	async regenerateKey() {
		this.confirmationDialog.nativeElement.close();
		this.regeneratingKey = true;
		try {
			const response = await this.createProjectService.regenerateKey();
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.confirmRegenerateKey = false;
			this.regeneratingKey = false;
			this.apiKey = response.data.key;
			this.tokenDialog.nativeElement.showModal();
			return;
		} catch (error) {
			this.regeneratingKey = false;
			return error;
		}
	}

	async createNewSignature(i: number) {
		if (this.newSignatureForm.invalid) return this.newSignatureForm.markAllAsTouched();

		this.versions.at(i).patchValue(this.newSignatureForm.value);
		await this.updateProject();
		this.newSignatureForm.reset();
		this.newSignatureDialog.nativeElement.showModal();
	}

	checkProjectConfig() {
		const configDetails = this.projectForm.value.config;
		const configKeys = Object.keys(configDetails).slice(0, -1);
		configKeys.forEach(configKey => {
			const configKeyValues = configDetails[configKey] ? Object.values(configDetails[configKey]) : [];
			if (configKeyValues.every(item => item === null)) delete this.projectForm.value.config[configKey];
		});

		if (this.projectForm.value.config.is_retention_policy_enabled === null) delete this.projectForm.value.config.is_retention_policy_enabled;
	}

	checkMetaEventsConfig() {
		const is_meta_events_enabled = this.projectForm.value.config.meta_event.is_enabled;
		const metaEventsConfig = Object.keys(this.projectForm.value.config.meta_event).slice(1, -1);
		if (!is_meta_events_enabled) {
			metaEventsConfig.forEach(config => {
				this.projectForm.get(`config.meta_event.${config}`)?.clearValidators();
				this.projectForm.get(`config.meta_event.${config}`)?.setErrors(null);
				this.projectForm.updateValueAndValidity();
			});
		}
	}

	getTimeString(timeValue: number) {
		if (timeValue > 59) return `${Math.round(timeValue / 60)}m`;
		return `${timeValue}s`;
	}

	getTimeValue(timeValue: any) {
		const [digits, word] = timeValue.match(/\D+|\d+/g);
		if (word === 's') return parseInt(digits);
		else if (word === 'm') return parseInt(digits) * 60;
		return parseInt(digits);
	}

	cancel() {
		this.confirmationDialog.nativeElement.showModal();
		document.getElementById('projectForm')?.scroll({ top: 0, behavior: 'smooth' });
	}

	confirmToggleAction(event: any, actionType?: 'metaEvents' | 'endpoints') {
		const disableValue = event.target.checked;
		if (actionType !== 'metaEvents') disableValue ? this.updateProject() : this.disableEndpointsDialog.nativeElement.showModal();
		else if (!disableValue && actionType === 'metaEvents') this.metaEventsDialog.nativeElement.showModal();
	}

	switchTab(tab: string) {
		if (tab === 'meta events') this.projectForm.patchValue({ config: { meta_event: { type: 'http' } } });
		this.activeTab = tab;
		this.addPageToUrl();
	}

	addPageToUrl() {
		const queryParams: any = {};
		queryParams.activePage = this.activeTab;
		this.router.navigate([], { queryParams: Object.assign({}, queryParams) });
	}

	get shouldShowBorder(): number {
		return this.configurations.filter(config => config.show).length;
	}
}
