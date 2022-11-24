import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { FormArray, FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute, Router } from '@angular/router';
import { APP } from 'src/app/models/endpoint.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { CreateAppService } from './create-app.service';

interface endpoint {
	url: string;
	description: string;
	secret: string;
	http_timeout: string;
	authentication?: {
		type: string;
		api_key: {
			header_name: string;
			header_value: string;
		};
	};
}

@Component({
	selector: 'app-create-app',
	templateUrl: './create-app.component.html',
	styleUrls: ['./create-app.component.scss']
})
export class CreateAppComponent implements OnInit {
	@Input() editAppMode: boolean = false;

	@Output() discardApp = new EventEmitter<any>();
	@Output() createApp = new EventEmitter<any>();
	appUid = this.route.snapshot.params.id;
	isSavingApp = false;
	isLoadingAppDetails = false;
	confirmModal = false;
	appsDetailsItem!: APP;
	addNewAppForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		support_email: [''],
		slack_webhook_url: [''],
		description: [''],
		is_disabled: [false],
		endpoints: this.formBuilder.array([])
	});
	constructor(private formBuilder: FormBuilder, private createAppService: CreateAppService, private generalService: GeneralService, private route: ActivatedRoute, private router: Router) {}

	async ngOnInit() {
		if (!this.editAppMode) {
			this.endpoints.push(this.newEndpoint());
			this.getApps();
		}
		if (this.editAppMode) await this.getAppDetails();
	}

	get endpoints(): FormArray {
		return this.addNewAppForm.get('endpoints') as FormArray;
	}

	getSingleEndpoint(index: any) {
		return ((this.addNewAppForm.get('endpoints') as FormArray)?.controls[index] as FormGroup)?.controls;
	}

	newEndpoint(): FormGroup {
		return this.formBuilder.group({
			url: ['', Validators.required],
			description: ['', Validators.required],
			secret: [null],
			http_timeout: [null],
			authentication: this.formBuilder.group({
				type: ['api_key'],
				api_key: this.formBuilder.group({
					header_name: [''],
					header_value: ['']
				})
			}),
			advanced_signatures: [null]
		});
	}

	addEndpoint() {
		this.endpoints.push(this.newEndpoint());
	}

	removeEndpoint(i: number) {
		this.endpoints.removeAt(i);
	}

	async saveApp() {
		if (this.editAppMode) delete this.addNewAppForm.value.endpoints;

		if (this.addNewAppForm.invalid) return this.addNewAppForm.markAsTouched();
		this.isSavingApp = true;
		let requests: any[] = [];

		try {
			const response = this.editAppMode ? await this.createAppService.updateApp({ appId: this.appsDetailsItem?.uid, body: this.addNewAppForm.value }) : await this.createAppService.createApp({ body: this.addNewAppForm.value });

			if (!this.editAppMode) {
				this.appUid = response.data.uid;
				const endpointData = this.addNewAppForm.value.endpoints;
				endpointData.forEach((item: endpoint) => {
					if (!item.authentication?.api_key.header_name && !item.authentication?.api_key.header_value) delete item.authentication;
					requests.push(this.addNewEndpoint(item));
				});
				this.saveNewEndpoints(requests);
			}
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.addNewAppForm.reset();
			this.createApp.emit(response.data);
			document.getElementById('configureProjectForm')?.scroll({ top: 0, behavior: 'smooth' });
			this.addNewAppForm.patchValue({
				is_disabled: false
			});
			this.isSavingApp = false;
			this.editAppMode = false;
			return;
		} catch (error) {
			this.isSavingApp = false;
			return;
		}
	}

	async addNewEndpoint(endpoint: any) {
		try {
			const response = await this.createAppService.addNewEndpoint({ appId: this.appUid, body: endpoint });

			return response;
		} catch {
			return;
		}
	}

	saveNewEndpoints(requests: any[]) {
		Promise.allSettled(requests);
	}

	async getAppDetails() {
		this.isLoadingAppDetails = true;

		try {
			const response = await this.createAppService.getApp(this.appUid);
			this.appsDetailsItem = response.data;
			this.addNewAppForm.patchValue(response.data);
			this.isLoadingAppDetails = false;
		} catch {
			this.isLoadingAppDetails = false;
		}
	}

	async getApps() {
		this.isLoadingAppDetails = true;
		try {
			const response = await this.createAppService.getApps();
			const apps = response.data.content;
			if (apps.length > 0 && this.router.url.includes('/configure')) this.createApp.emit();
			this.isLoadingAppDetails = false;
		} catch {
			this.isLoadingAppDetails = false;
		}
	}

	cancel() {
		document.getElementById(this.router.url.includes('/configure') ? 'configureProjectForm' : 'appForm')?.scroll({ top: 0, behavior: 'smooth' });
		this.confirmModal = true;
	}

	isNewProjectRoute(): boolean {
		if (this.router.url == '/projects/new') return true;
		return false;
	}
}
