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
	eventTags!: string[];
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
			events: [''],
			tag: ['', Validators.required],
			description: ['', Validators.required]
		});
	}

	addEndpoint() {
		this.endpoints.push(this.newEndpoint());
	}

	removeEndpoint(i: number) {
		this.endpoints.removeAt(i);
	}

	removeEventTag(tag: string) {
		this.eventTags = this.eventTags.filter(e => e !== tag);
	}

	addTag() {
		const addTagInput = document.getElementById('tagInput');
		const addTagInputValue = document.getElementById('tagInput') as HTMLInputElement;
		addTagInput?.addEventListener('keydown', e => {
			if (e.which === 188) {
				if (this.eventTags.includes(addTagInputValue?.value)) {
					addTagInputValue.value = '';
					this.eventTags = this.eventTags.filter(e => String(e).trim());
				} else {
					this.eventTags.push(addTagInputValue?.value);
					addTagInputValue.value = '';
					this.eventTags = this.eventTags.filter(e => String(e).trim());
				}
				e.preventDefault();
			}
		});
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
		// to be reviewed
		delete this.addNewAppForm.value.endpoints;

		try {
			const response = this.editAppMode
				? await this.createAppService.updateApp({ appId: this.appsDetailsItem?.uid, body: this.addNewAppForm.value })
				: await this.createAppService.createApp({ body: this.addNewAppForm.value });

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

	closeAppInstance() {
		this.discardApp.emit();
	}
}
