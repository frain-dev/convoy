import { Component, EventEmitter, Input, OnInit, Output, inject } from '@angular/core';
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
import { FormLoaderComponent } from 'src/app/components/form-loader/form-loader.component';
import { EndpointDetailsService } from '../../pages/project/endpoint-details/endpoint-details.service';
import { PermissionDirective } from '../permission/permission.directive';
import { RbacService } from 'src/app/services/rbac/rbac.service';
import { ENDPOINT } from 'src/app/models/endpoint.model';

@Component({
	selector: 'convoy-create-endpoint',
	standalone: true,
	imports: [CommonModule, ReactiveFormsModule, InputDirective, InputErrorComponent, InputFieldDirective, LabelComponent, ButtonComponent, RadioComponent, TooltipComponent, CardComponent, ToggleComponent, FormLoaderComponent, PermissionDirective],
	templateUrl: './create-endpoint.component.html',
	styleUrls: ['./create-endpoint.component.scss']
})
export class CreateEndpointComponent implements OnInit {
	@Input('editMode') editMode = false;
	@Input('showAction') showAction: 'true' | 'false' = 'false';
	@Input('type') type: 'in-app' | 'portal' = 'in-app';
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
		http_timeout: [null, Validators.pattern('^[-+]?[0-9]+$')],
		description: [null],
		owner_id: [null],
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
	@Input('endpointId') endpointUid: string = this.route.snapshot.params.id;
	enableMoreConfig = false;
	configurations = [{ uid: 'http_timeout', name: 'Endpoint Timeout ', show: false }];
	endpointCreated: boolean = false;
	private rbacService = inject(RbacService);

	constructor(
		private formBuilder: FormBuilder,
		private generalService: GeneralService,
		private createEndpointService: CreateEndpointService,
		private route: ActivatedRoute,
		public privateService: PrivateService,
		private router: Router,
		private endpointService: EndpointDetailsService
	) {}

	async ngOnInit() {
		if (this.type !== 'portal') this.configurations.push({ uid: 'alert-config', name: 'Alert Configuration', show: false }, { uid: 'auth', name: 'Authentication', show: false }, { uid: 'signature', name: 'Signature Format', show: false });
		if (this.endpointUid && this.editMode) this.getEndpointDetails();
		if (!(await this.rbacService.userCanAccess('Endpoints|MANAGE'))) this.addNewEndpointForm.disable();
	}

	async saveEndpoint() {
		if (this.addNewEndpointForm.invalid) return this.addNewEndpointForm.markAllAsTouched();

		this.savingEndpoint = true;

		if (!this.addNewEndpointForm.value.authentication.api_key.header_name && !this.addNewEndpointForm.value.authentication.api_key.header_value) delete this.addNewEndpointForm.value.authentication;

		try {
			const response = this.endpointUid && this.editMode ? await this.createEndpointService.editEndpoint({ endpointId: this.endpointUid || '', body: this.addNewEndpointForm.value }) : await this.createEndpointService.addNewEndpoint({ body: this.addNewEndpointForm.value });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.onAction.emit({ action: this.endpointUid && this.editMode ? 'update' : 'save', data: response.data });
			this.addNewEndpointForm.reset();
			this.endpointCreated = true;
			return response;
		} catch {
			this.endpointCreated = false;
			this.savingEndpoint = false;
			return;
		}
	}

	async getEndpointDetails() {
		this.isLoadingEndpointDetails = true;

		try {
			const response = await this.endpointService.getEndpoint(this.endpointUid);
			const endpointDetails: ENDPOINT = response.data;
			this.addNewEndpointForm.patchValue(endpointDetails);
			this.addNewEndpointForm.patchValue({
				name: endpointDetails.title,
				url: endpointDetails.target_url
			});

			if (endpointDetails.support_email) this.toggleConfigForm('alert-config');
			if (endpointDetails.authentication.api_key.header_value || endpointDetails.authentication.api_key.header_name) this.toggleConfigForm('auth');
			if (endpointDetails.http_timeout) this.toggleConfigForm('http_timeout');

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

	toggleConfigForm(configValue: string) {
		this.configurations.forEach(config => {
			if (config.uid === configValue) config.show = !config.show;
		});
	}

	showConfig(configValue: string): boolean {
		return this.configurations.find(config => config.uid === configValue)?.show || false;
	}

	cancel() {
		this.onAction.emit({ action: 'close' });
	}
}
