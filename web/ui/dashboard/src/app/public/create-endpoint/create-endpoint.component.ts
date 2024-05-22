import { Component, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateEndpointComponent } from '../../private/components/create-endpoint/create-endpoint.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { RouterModule } from '@angular/router';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { CreateEndpointService } from 'src/app/private/components/create-endpoint/create-endpoint.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { SUBSCRIPTION } from 'src/app/models/subscription';
import { ENDPOINT } from 'src/app/models/endpoint.model';

interface PORTAL_ENDPOINT extends ENDPOINT {
	subscription?: SUBSCRIPTION;
}

@Component({
	selector: 'convoy-create-portal-endpoint',
	standalone: true,
	imports: [CommonModule, CreateEndpointComponent, ButtonComponent, RouterModule, TooltipComponent],
	templateUrl: './create-endpoint.component.html',
	styleUrls: ['./create-endpoint.component.scss']
})
export class CreatePortalEndpointComponent implements OnInit {
	@ViewChild(CreateEndpointComponent) createEndpointForm!: CreateEndpointComponent;
	@Output('onAction') onAction = new EventEmitter();
	@Input('endpoint') endpoint?: PORTAL_ENDPOINT;

	isCreatingEndpoint = false;

	constructor(private endpointService: CreateEndpointService, private generalService: GeneralService) {}

	ngOnInit(): void {}

	async createEndpoint() {
		this.isCreatingEndpoint = true;
		if (this.createEndpointForm.addNewEndpointForm.invalid) return this.createEndpointForm.addNewEndpointForm.markAllAsTouched();

		const endpointFormValue = structuredClone(this.createEndpointForm.addNewEndpointForm.value);
		delete endpointFormValue.authentication;

		try {
			const endpointDetails = await this.endpointService.addNewEndpoint({ body: endpointFormValue });
			this.generalService.showNotification({ message: endpointDetails.message, style: 'success' });
			this.onAction.emit({ action: 'create' });
			this.isCreatingEndpoint = false;
		} catch (error) {
			this.isCreatingEndpoint = false;
		}
	}

	async updateEndpoint() {
		this.isCreatingEndpoint = true;
		if (this.createEndpointForm.addNewEndpointForm.invalid) return this.createEndpointForm.addNewEndpointForm.markAllAsTouched();

		const endpointFormValue = structuredClone(this.createEndpointForm.addNewEndpointForm.value);
		delete endpointFormValue.authentication;

		try {
			await this.endpointService.editEndpoint({ endpointId: this.endpoint?.uid || '', body: endpointFormValue });

			this.generalService.showNotification({ message: 'Endpint updated successfully', style: 'success' });
			this.onAction.emit({ action: 'create' });
			this.isCreatingEndpoint = false;
		} catch (error) {
			this.isCreatingEndpoint = false;
		}
	}
}
