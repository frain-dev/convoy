import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
// import { InputComponent } from 'src/app/components/input/input.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { EndpointsService } from '../../pages/project/endpoints/endpoints.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { ENDPOINT } from 'src/app/models/app.model';

@Component({
	selector: 'send-event',
	standalone: true,
	imports: [CommonModule, ModalComponent, SelectComponent, InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent, ButtonComponent, ReactiveFormsModule],
	templateUrl: './send-event.component.html',
	styleUrls: ['./send-event.component.scss']
})
export class SendEventComponent implements OnInit {
	@Input() endpointId!: string;
	@Output() onAction = new EventEmitter<any>();
	isSendingNewEvent = false;
	sendEventForm: FormGroup = this.formBuilder.group({
		endpoint_id: ['', Validators.required],
		data: ['', Validators.required],
		event_type: ['', Validators.required]
	});
	endpoints!: ENDPOINT[];
	constructor(private formBuilder: FormBuilder, private router: Router, private endpointService: EndpointsService, private generalService: GeneralService) {}

	ngOnInit() {
		this.getEndpoints();
		this.setEventEndpointId();
	}

	cancel() {
		this.onAction.emit({ action: 'cancel' });
	}

	async sendNewEvent() {
		if (this.sendEventForm.invalid) return this.sendEventForm.markAsTouched();

		if (!this.convertStringToJson(this.sendEventForm.value.data)) return;
		this.sendEventForm.value.data = this.convertStringToJson(this.sendEventForm.value.data);

		this.isSendingNewEvent = true;
		try {
			const response = await this.endpointService.sendEvent({ body: this.sendEventForm.value });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.sendEventForm.reset();
			this.onAction.emit({ action: 'sentEvent' });
			this.isSendingNewEvent = false;
			this.router.navigate(['/projects/' + this.endpointService.projectId + '/events'], { queryParams: { eventsApp: this.endpointId } });
		} catch {
			this.isSendingNewEvent = false;
		}
	}

	async getEndpoints() {
		try {
			const response = await this.endpointService.getEndpoints();
			this.endpoints = response.data.content;
			console.log(response);
		} catch {}
	}

	setEventEndpointId() {
		this.sendEventForm.patchValue({
			endpoint_id: this.endpointId
		});
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
