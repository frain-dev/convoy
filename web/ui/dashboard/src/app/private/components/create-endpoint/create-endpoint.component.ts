import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { RadioComponent } from 'src/app/components/radio/radio.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { ActivatedRoute } from '@angular/router';
import { CardComponent } from 'src/app/components/card/card.component';
import { CreateEndpointService } from './create-endpoint.service';

@Component({
	selector: 'convoy-create-endpoint',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent, ButtonComponent, RadioComponent, TooltipComponent, CardComponent],
	templateUrl: './create-endpoint.component.html',
	styleUrls: ['./create-endpoint.component.scss']
})
export class CreateEndpointComponent implements OnInit {
	@Input() appId!: string;
	@Input() selectedEndpoint?: ENDPOINT;
	@Output() onAction = new EventEmitter<any>();
	savingEndpoint = false;
	isLoadingEndpointDetails = false;
	addNewEndpointForm: FormGroup = this.formBuilder.group({
		name: ['', Validators.required],
		support_email: [],
		slack_webhook_url: [],
		url: ['', Validators.required],
		secret: [null],
		http_timeout: [null],
		description: ['', Validators.required],
		authentication: this.formBuilder.group({
			type: ['api_key'],
			api_key: this.formBuilder.group({
				header_name: [''],
				header_value: ['']
			})
		}),
		advanced_signatures: [null, Validators.required]
	});
	token: string = this.route.snapshot.params.token;
	endpointUid: string = this.route.snapshot.params.id;

	constructor(private formBuilder: FormBuilder, private generalService: GeneralService, private createEndpointService: CreateEndpointService, private route: ActivatedRoute) {}

	ngOnInit() {
		if (this.endpointUid) this.getEndpointDetails();
	}

	async saveEndpoint() {
		console.log(this.addNewEndpointForm.value);
		if (this.addNewEndpointForm.invalid) return this.addNewEndpointForm.markAsTouched();
		this.savingEndpoint = true;

		if (!this.addNewEndpointForm.value.authentication.api_key.header_name && !this.addNewEndpointForm.value.authentication.api_key.header_value) delete this.addNewEndpointForm.value.authentication;

		try {
			const response = this.endpointUid
				? await this.createEndpointService.editEndpoint({  endpointId: this.endpointUid || '', body: this.addNewEndpointForm.value, token: this.token })
				: await this.createEndpointService.addNewEndpoint({ body: this.addNewEndpointForm.value, token: this.token });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.onAction.emit({ action: this.selectedEndpoint ? 'update' : 'save', data: response.data });
			this.addNewEndpointForm.reset();
			this.savingEndpoint = false;
			return;
		} catch {
			this.savingEndpoint = false;
			return;
		}
	}

	async getEndpointDetails() {
		this.isLoadingEndpointDetails = true;

		try {
			const response = await this.createEndpointService.getEndpoint(this.endpointUid);
			const endpointDetails = response.data;
			console.log(endpointDetails);
			// this.addNewAppForm.patchValue(response.data);
			this.addNewEndpointForm.patchValue(endpointDetails);
			this.addNewEndpointForm.patchValue({
				name: endpointDetails.title,
				url: endpointDetails.target_url
			});
			this.isLoadingEndpointDetails = false;
		} catch {
			this.isLoadingEndpointDetails = false;
		}
	}

	updateEndpointForm() {
		if (this.selectedEndpoint) {
			this.addNewEndpointForm.patchValue(this.selectedEndpoint);
			this.addNewEndpointForm.patchValue({
				url: this.selectedEndpoint.target_url
			});
		}
	}

	cancel() {
		this.onAction.emit({ action: 'close' });
	}
}
