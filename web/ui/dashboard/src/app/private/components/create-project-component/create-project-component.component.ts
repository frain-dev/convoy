import { Component, ElementRef, EventEmitter, inject, Input, OnInit, Output, ViewChild } from '@angular/core';
import { FormArray, FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { PROJECT, VERSIONS } from 'src/app/models/project.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { PrivateService } from '../../private.service';
import { CreateProjectComponentService } from './create-project-component.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { EVENT_TYPE } from 'src/app/models/event.model';
import { AbstractControl, ValidationErrors, ValidatorFn } from '@angular/forms';


interface TAB {
	label: string;
	svg: 'fill' | 'stroke';
	icon: string;
}

function jsonValidator(): ValidatorFn {
    return (control: AbstractControl): ValidationErrors | null => {
        if (!control.value || typeof control.value === 'object') {
            return null;
        }
        try {
            JSON.parse(control.value);
            return null; // Valid JSON
        } catch (e) {
            return { invalidJson: true }; // Invalid JSON
        }
    };
}

@Component({
	selector: 'app-create-project-component',
	templateUrl: './create-project-component.component.html',
	styleUrls: ['./create-project-component.component.scss']
})
export class CreateProjectComponent implements OnInit {
	@ViewChild('disableEndpointsDialog', { static: true }) disableEndpointsDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('disableTLSEndpointsDialog', { static: true }) disableTLSEndpointsDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('metaEventsDialog', { static: true }) metaEventsDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('confirmationDialog', { static: true }) confirmationDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('newSignatureDialog', { static: true }) newSignatureDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('newEventTypeDialog', { static: true }) newEventTypeDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('mutliSubEndpointsDialog', { static: true }) mutliSubEndpointsDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('tokenDialog', { static: true }) tokenDialog!: ElementRef<HTMLDialogElement>;

	signatureTableHead: string[] = ['Header', 'Version', 'Hash', 'Encoding'];
	eventTypeTableHead: string[] = ['Event Type', 'Category', 'Description', ''];
	projectForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		config: this.formBuilder.group({
			search_policy: [720],
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
			ssl: this.formBuilder.group({
				enforce_secure_endpoints: [true]
			}),
			disable_endpoint: [false, Validators.required],
			multiple_endpoint_subscriptions: [false, Validators.required],
			meta_event: this.formBuilder.group({
				is_enabled: [false, Validators.required],
				type: ['http', Validators.required],
				event_type: [[], Validators.required],
				url: ['', Validators.required],
				secret: [null]
			})
		}),
		type: [null, Validators.required]
	});
	newEventTypeForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		category: ['', Validators.required],
		description: ['', Validators.required],
        json_schema: ['', jsonValidator()],
	});
	newSignatureForm: FormGroup = this.formBuilder.group({
		encoding: [null],
		hash: [null]
	});
	isCreatingProject = false;
	enableMoreConfig = false;
	regeneratingKey = false;
	apiKey!: string;
	hashAlgorithms = ['SHA256', 'SHA512'];
	retryLogicTypes = [
		{ uid: 'linear', name: 'Linear time retry' },
		{ uid: 'exponential', name: 'Exponential time backoff' }
	];
	encodings = ['base64', 'hex'];
	@Output('onAction') onAction = new EventEmitter<any>();
	@Input('action') action: 'create' | 'update' = 'create';
	projectDetails!: PROJECT;
	signatureVersions!: { date: string; content: VERSIONS[] }[];
	configurations = [
		{ uid: 'strategy', name: 'Retry Config', show: false, deleted: false },
		{ uid: 'ratelimit', name: 'Rate Limit', show: false, deleted: false },
		{ uid: 'search_policy', name: 'Search Policy', show: false, deleted: false },
		{ uid: 'signature', name: 'Signature Format', show: false, deleted: false },
	];
	public rbacService = inject(RbacService);
	tabs: TAB[] = [
		{ label: 'project config', svg: 'fill', icon: 'settings' },
		{ label: 'signature history', svg: 'fill', icon: 'sig-history' },
		{ label: 'endpoints config', svg: 'stroke', icon: 'endpoints' },
		{ label: 'meta events config', svg: 'stroke', icon: 'meta-events' },
		{ label: 'event types', svg: 'stroke', icon: 'event-type' },
		{ label: 'secrets', svg: 'stroke', icon: 'secret' }
	];
	activeTab = this.tabs[0];
	events = ['endpoint.created', 'endpoint.deleted', 'endpoint.updated', 'eventdelivery.success', 'eventdelivery.failed', 'project.updated'];
	eventTypes: EVENT_TYPE[] = [];
	selectedEventType: EVENT_TYPE | null = null;
    rateLimitDeleted = false;

	constructor(
		private formBuilder: FormBuilder,
		private createProjectService: CreateProjectComponentService,
		private generalService: GeneralService,
		private privateService: PrivateService,
		public router: Router,
		private route: ActivatedRoute,
		public licenseService: LicensesService
	) {}

	async ngOnInit() {
		this.getEventTypes();
		if (this.action === 'update') this.getProjectDetails();
		if (!(await this.rbacService.userCanAccess('Project Settings|MANAGE'))) this.projectForm.disable();
		if (this.action === 'update') this.switchTab(this.tabs.find(tab => tab.label == this.route.snapshot.queryParams?.activePage) ?? this.tabs[0]);
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

	toggleConfigForm(configValue: string, deleted?: boolean) {
		this.configurations.forEach(config => {
			if (config.uid === configValue) {
                config.show = !config.show;
                config.deleted = deleted ?? false;
            }
		});
	}
	setConfigFormDeleted(configValue: string, deleted: boolean) {
		this.configurations.forEach(config => {
			if (config.uid === configValue) {
                config.deleted = deleted;
            }
		});
	}

	showConfig(configValue: string): boolean {
		return this.configurations.find(config => config.uid === configValue)?.show || false;
	}

    configDeleted(configValue: string): boolean {
        return this.configurations.find(config => config.uid === configValue)?.deleted || false;
    }

	async getProjectDetails() {
		try {
			this.projectDetails = this.privateService.getProjectDetails;

			this.setSignatureVersions();

			if (this.projectDetails?.type === 'incoming') this.tabs = this.tabs.filter(tab => tab.label !== 'signature history' && tab.label !== 'event types');

			this.projectForm.patchValue(this.projectDetails);
			this.projectForm.get('config.strategy')?.patchValue(this.projectDetails.config.strategy);
			this.projectForm.get('config.signature')?.patchValue(this.projectDetails.config.signature);
			this.projectForm.get('config.ratelimit')?.patchValue(this.projectDetails.config.ratelimit);
			this.projectForm.get('config.search_policy')?.patchValue(this.getHours(this.projectDetails.config.search_policy));

			// set meta events config
			this.projectDetails.config.meta_event && this.projectDetails.config.meta_event.is_enabled
				? this.projectForm.get('config.meta_event.is_enabled')?.patchValue(this.projectDetails.config.meta_event.is_enabled)
				: this.projectForm.get('config.meta_event.is_enabled')?.patchValue(false);

			this.projectForm.get('config.meta_event.type')?.patchValue('http');

			let filteredConfigs: string[] = [];
			if (this.projectDetails?.type === 'incoming') filteredConfigs.push('signature');

			this.configurations.filter(item => !filteredConfigs.includes(item.uid)).forEach(config => this.toggleConfigForm(config.uid));
		} catch {}
	}

	setSignatureVersions() {
		const versions = this.projectDetails.config.signature.versions;
		if (!versions?.length) return;
		this.signatureVersions = this.generalService.setContentDisplayed(versions);
		versions.forEach((version: { encoding: any; hash: any }, index: number) => {
			this.addVersion();
			this.versions.at(index)?.patchValue({
				encoding: version.encoding,
				hash: version.hash
			});
		});
	}

	async createProject() {
		const projectFormModal = document.getElementById('projectForm');

		if (this.projectForm.get('name')?.invalid || this.projectForm.get('type')?.invalid) {
			projectFormModal?.scroll({ top: 0 });
			this.projectForm.markAllAsTouched();
			return;
		}
		const projectData = this.getProjectData();

		this.isCreatingProject = true;

		try {
			// this createProject service also updates the project as an active project in localstorage
			const response = await this.createProjectService.createProject(projectData);
			await this.privateService.getProjectStat({ refresh: true });

			this.privateService.getProjects({ refresh: true });

			projectFormModal?.scroll({ top: 0, behavior: 'smooth' });
			this.isCreatingProject = false;
			this.projectForm.reset();
			this.apiKey = response.data.api_key.key;
			this.projectDetails = response.data.project;
			if (projectFormModal) projectFormModal.style.overflowY = 'hidden';
			this.tokenDialog.nativeElement.showModal();
			this.licenseService.setLicenses();
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
		if (typeof this.projectForm.value.config.search_policy === 'number') this.projectForm.value.config.search_policy = `${this.projectForm.value.config.search_policy}h`;


        if (!this.showConfig('ratelimit') && this.configDeleted('ratelimit')) {
            this.projectForm.value.config.ratelimit.count = 0;
            this.projectForm.value.config.ratelimit.duration = 0;
            this.projectForm.value.config.ratelimit = null;

            this.projectForm.get('config.ratelimit')?.patchValue({ count: 0, duration: 0 });

            this.setConfigFormDeleted('ratelimit', false);
        }

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
		this.newSignatureDialog.nativeElement.close();
		this.getProjectDetails();
	}

	getProjectData() {
		const configKeys = Object.keys(this.projectForm.value.config);
		const projectData = this.projectForm.value;

		configKeys.forEach(configKey => {
			if (!this.showConfig(configKey)) delete projectData.config[configKey];
			if (this.showConfig('search_policy') && typeof projectData.config.search_policy === 'number') projectData.config.search_policy = `${projectData.config.search_policy}h`;
		});

		return projectData;
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

	getHours(hours: any) {
		const [digits, _] = hours.match(/\D+|\d+/g);
		return parseInt(digits);
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

	async confirmToggleAction(event: any, actionType?: 'metaEvents' | 'endpoints') {
		const disableValue = event.target.checked;
		if (actionType === 'endpoints') disableValue ? await this.updateProject() : this.disableEndpointsDialog.nativeElement.showModal();
		else if (!disableValue && actionType === 'metaEvents') this.metaEventsDialog.nativeElement.showModal();
	}

	confirmTLSToggleAction(event: any) {
		const disableValue = event.target.checked;
		disableValue ? this.updateProject() : this.disableTLSEndpointsDialog.nativeElement.showModal();
	}

	switchTab(tab: TAB) {
		if (tab.label === 'meta events') this.projectForm.patchValue({ config: { meta_event: { type: 'http' } } });
		this.activeTab = tab;
		this.addPageToUrl();
	}

	addPageToUrl() {
		const queryParams: any = {};
		queryParams.activePage = this.activeTab.label;
		this.router.navigate([], { queryParams: Object.assign({}, queryParams) });
	}

	get shouldShowBorder(): number {
		return this.configurations.filter(config => config.show).length;
	}

    async createNewEventType() {
        try {
            const payload = {
                ...this.newEventTypeForm.value,
                json_schema: this.newEventTypeForm.value.json_schema
                    ? (typeof this.newEventTypeForm.value.json_schema === 'string'
                        ? JSON.parse(this.newEventTypeForm.value.json_schema)
                        : this.newEventTypeForm.value.json_schema)
                    : {},
            };

            await this.createProjectService.createEventType(payload);
            this.newEventTypeForm.reset();
            this.newEventTypeDialog.nativeElement.close();
            this.getEventTypes();
        } catch (error) {
            console.error("Error creating event type:", error);
        }
    }

    async updateNewEventType() {
        if (!this.selectedEventType) {
            this.generalService.showNotification({message: "Event type not selected", style: 'error', type: 'alert'});
            return;
        }

        const payload = {
            data: {
                ...this.newEventTypeForm.value,
                json_schema: this.newEventTypeForm.value.json_schema
                    ? (typeof this.newEventTypeForm.value.json_schema === 'string'
                        ? JSON.parse(this.newEventTypeForm.value.json_schema)
                        : this.newEventTypeForm.value.json_schema)
                    : {},
            },
            eventId: this.selectedEventType.uid
        };

        try {
            await this.createProjectService.updateEventType(payload);
            this.newEventTypeForm.reset();
            this.newEventTypeDialog.nativeElement.close();
            this.getEventTypes();
        } catch (error) {
            console.error("Error updating event type:", error);
        }
    }

	async deprecateEventType(eventTypeId: string) {
		try {
			await this.createProjectService.deprecateEventType(eventTypeId);
			this.getEventTypes();
		} catch {}
	}

	async getEventTypes() {
		if (this.privateService.getProjectDetails?.type === 'incoming') return;

		try {
			const response = await this.privateService.getEventTypes();
			this.eventTypes = response.data;
			return;
		} catch (error) {
			return;
		}
	}

	openEditEventTypeModal(eventType: any) {
		this.selectedEventType = eventType;
		this.newEventTypeForm.patchValue({
			name: eventType.name,
			description: eventType.description,
			category: eventType.category,
            json_schema: eventType.json_schema
                ? (typeof eventType.json_schema === 'string'
                    ? eventType.json_schema
                    : JSON.stringify(eventType.json_schema))
                : ''
		});
		this.newEventTypeDialog.nativeElement.showModal();
	}

    async importOpenAPISpec(event: Event) {
        const file = (event.target as HTMLInputElement).files?.[0];
        if (!file) return;

        const reader = new FileReader();
        reader.onload = async () => {
            try {
                const specContent = reader.result as string;
                await this.createProjectService.importOpenAPISpec({ spec: specContent });
                this.generalService.showNotification({ message: "OpenAPI spec imported successfully", style: 'success' });
                this.getEventTypes(); // Refresh event types
            } catch (error) {
                this.generalService.showNotification({ message: "Failed to import OpenAPI spec", style: 'error' });
                console.error("Import error:", error);
            }
        };
        reader.readAsText(file);
    }
}
