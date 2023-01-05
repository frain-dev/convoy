import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormBuilder, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
import { InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent } from 'src/app/components/input/input.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { RadioComponent } from 'src/app/components/radio/radio.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { ActivatedRoute, Router } from '@angular/router';
import { CardComponent } from 'src/app/components/card/card.component';
import { CreateEndpointService } from './create-endpoint.service';
import { PrivateService } from '../../private.service';
import { ToggleComponent } from 'src/app/components/toggle/toggle.component';

@Component({
	selector: 'convoy-create-endpoint',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent, ButtonComponent, RadioComponent, TooltipComponent, CardComponent, ToggleComponent],
	templateUrl: './create-endpoint.component.html',
	styleUrls: ['./create-endpoint.component.scss']
})
export class CreateEndpointComponent implements OnInit {
	@Input('editMode') editMode = false;
	@Output() onAction = new EventEmitter<any>();
	savingEndpoint = false;
	isLoadingEndpointDetails = false;
	isLoadingEndpoints = false;
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
		advanced_signatures: [false, Validators.required]
	});
	token: string = this.route.snapshot.params.token;
	endpointUid: string = this.route.snapshot.params.id;
	enableMoreConfig = false;

	constructor(private formBuilder: FormBuilder, private generalService: GeneralService, private createEndpointService: CreateEndpointService, private route: ActivatedRoute, private privateService: PrivateService, private router: Router) {}

	ngOnInit() {
		if (!this.editMode) this.getEndpoints();
		if (this.endpointUid && this.editMode) this.getEndpointDetails();
	}

	async saveEndpoint() {
		if (this.addNewEndpointForm.invalid) return this.addNewEndpointForm.markAsTouched();
		this.savingEndpoint = true;

		if (!this.addNewEndpointForm.value.authentication.api_key.header_name && !this.addNewEndpointForm.value.authentication.api_key.header_value) delete this.addNewEndpointForm.value.authentication;

		try {
			const response =
				this.endpointUid && this.editMode
					? await this.createEndpointService.editEndpoint({ endpointId: this.endpointUid || '', body: this.addNewEndpointForm.value, token: this.token })
					: await this.createEndpointService.addNewEndpoint({ body: this.addNewEndpointForm.value, token: this.token });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.onAction.emit({ action: this.endpointUid && this.editMode ? 'update' : 'save', data: response.data });
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

	async getEndpoints() {
		this.isLoadingEndpoints = true;
		try {
			const response = await this.privateService.getEndpoints();
			const endpoints = response.data.content;
			if (endpoints.length > 0 && this.router.url.includes('/configure')) this.onAction.emit({ action: 'save' });
			this.isLoadingEndpoints = false;
		} catch {
			this.isLoadingEndpoints = false;
		}
	}

	cancel() {
		this.onAction.emit({ action: 'close' });
	}
}
