import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { GeneralService } from 'src/app/services/general/general.service';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { PrivateService } from '../../private.service';

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
	constructor(private formBuilder: FormBuilder, private privateService: PrivateService, private generalService: GeneralService) {}

	ngOnInit() {
		this.getEndpoints();
		if (this.endpointId) this.setEventEndpointId();
	}

	cancel() {
		this.onAction.emit({ action: 'cancel' });
	}

	sendNewEvent() {}

	async getEndpoints() {
		try {
			const response = await this.privateService.getEndpoints();
			const endpointData = response.data.content;
			endpointData.forEach((data: ENDPOINT) => {
				data.name = data.title;
			});
			this.endpoints = endpointData;
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
