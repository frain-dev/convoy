import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { FormArray, FormBuilder, FormControl, FormGroup, Validators } from '@angular/forms';
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
	@Input() appsDetailsItem!: APP;

	@Output() discardApp = new EventEmitter<any>();
	@Output() createApp = new EventEmitter<any>();
	appUid!: string;
	isSavingApp: boolean = false;
	addNewAppForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		support_email: [''],
		slack_webhook_url: [''],
		description: [''],
		is_disabled: [false],
		endpoints: this.formBuilder.array([])
	});
	constructor(private formBuilder: FormBuilder, private createAppService: CreateAppService, private generalService: GeneralService) {}

	ngOnInit(): void {
		if (this.appsDetailsItem && this.editAppMode) {
			this.updateForm();
		}
		this.endpoints.push(this.newEndpoint());
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
			http_timeout: [''],
			rate_limit: [''],
			rate_limit_duration: ['']
		});
	}

	addEndpoint() {
		this.endpoints.push(this.newEndpoint());
	}

	removeEndpoint(i: number) {
		this.endpoints.removeAt(i);
	}

	updateForm() {
		this.addNewAppForm.patchValue({
			name: this.appsDetailsItem?.name,
			support_email: this.appsDetailsItem?.support_email,
			is_disabled: this.appsDetailsItem?.is_disabled
		});
	}

	async saveApp() {
		if (this.addNewAppForm.invalid) {
			(<any>Object).values(this.addNewAppForm.controls).forEach((control: FormControl) => {
				control?.markAsTouched();
			});
			return;
		}

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
		Promise.all(requests);
	}

	closeAppInstance() {
		this.discardApp.emit();
	}
}
