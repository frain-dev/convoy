import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { APP } from 'src/app/models/app.model';
import { PAGINATION } from 'src/app/models/global.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { AppDetailsService } from '../app-details.service';

@Component({
	selector: 'send-event',
	standalone: true,
	imports: [CommonModule, ModalComponent, SelectComponent, ButtonComponent, ReactiveFormsModule, InputFieldDirective, InputErrorComponent, InputDirective, LabelComponent],
	templateUrl: './send-event.component.html',
	styleUrls: ['./send-event.component.scss']
})
export class SendEventComponent implements OnInit {
	@Input() appId!: string;
	@Output() onAction = new EventEmitter<any>();
	isSendingNewEvent = false;
	apps?: { pagination: PAGINATION; content: APP[] };
	sendEventForm: FormGroup = this.formBuilder.group({
		app_id: ['', Validators.required],
		data: ['', Validators.required],
		event_type: ['', Validators.required]
	});
	constructor(private formBuilder: FormBuilder, private router: Router, private appDetailsService: AppDetailsService, private generalService: GeneralService) {}

	ngOnInit() {
		this.getApps();
		this.setEventAppId();
	}

	cancel() {
		this.onAction.emit({ action: 'cancel' });
	}

	setEventAppId() {
		this.sendEventForm.patchValue({
			app_id: this.appId
		});
	}

	async sendNewEvent() {
		if (this.sendEventForm.invalid) return this.sendEventForm.markAsTouched();

		if (!this.convertStringToJson(this.sendEventForm.value.data)) return;
		this.sendEventForm.value.data = this.convertStringToJson(this.sendEventForm.value.data);

		this.isSendingNewEvent = true;
		try {
			const response = await this.appDetailsService.sendEvent({ body: this.sendEventForm.value });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.sendEventForm.reset();
			this.onAction.emit({ action: 'sentEvent' });
			this.isSendingNewEvent = false;
			this.router.navigate(['/projects/' + this.appDetailsService.projectId + '/events'], { queryParams: { eventsApp: this.appId } });
		} catch {
			this.isSendingNewEvent = false;
		}
	}

	async getApps() {
		try {
			const appsResponse = await this.appDetailsService.getApps();

			this.apps = appsResponse.data;
		} catch (error) {
			return error;
		}
	}

	convertStringToJson(str: string) {
		try {
			const jsonObject = JSON.parse(str);
			return jsonObject;
		} catch {
			this.generalService.showNotification({ message: 'Event data is not entered in correct JSON format', style: 'error' });
			return false;
		}
	}
}
