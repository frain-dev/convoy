import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { FormArray, FormBuilder, FormGroup, Validators } from '@angular/forms';
import { ActivatedRoute } from '@angular/router';
import { APP } from 'src/app/models/app.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { CreateAppService } from './create-app.service';

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
	appsDetailsItem!: APP;
	addNewAppForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		support_email: [''],
		slack_webhook_url: [''],
		description: [''],
		is_disabled: [false],
		endpoints: this.formBuilder.array([])
	});
	constructor(private formBuilder: FormBuilder, private createAppService: CreateAppService, private generalService: GeneralService, private route: ActivatedRoute) {}

	async ngOnInit() {
		if (!this.editAppMode) this.endpoints.push(this.newEndpoint());
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
			http_timeout: [null]
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
			const response = this.editAppMode
				? await this.createAppService.updateApp({ appId: this.appsDetailsItem?.uid, body: this.addNewAppForm.value })
				: await this.createAppService.createApp({ body: this.addNewAppForm.value });

			if (!this.editAppMode) {
				this.appUid = response.data.uid;
				const endpointData = this.addNewAppForm.value.endpoints;
				endpointData.forEach((item: any) => {
					requests.push(this.addNewEndpoint(item));
				});
				this.saveNewEndpoints(requests);
			}
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.addNewAppForm.reset();
			this.createApp.emit(response.data);
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

	closeAppInstance() {
		this.discardApp.emit();
	}
}
