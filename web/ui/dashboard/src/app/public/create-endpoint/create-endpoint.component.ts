import { Component, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateEndpointComponent } from '../../private/components/create-endpoint/create-endpoint.component';
import { CreateSubscriptionModule } from 'src/app/private/components/create-subscription/create-subscription.module';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CreateSubscriptionService } from 'src/app/private/components/create-subscription/create-subscription.service';
import { CreateSubscriptionComponent } from 'src/app/private/components/create-subscription/create-subscription.component';
import { RouterModule } from '@angular/router';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { CreateEndpointService } from 'src/app/private/components/create-endpoint/create-endpoint.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { SUBSCRIPTION } from 'src/app/models/subscription';

@Component({
	selector: 'convoy-create-portal-endpoint',
	standalone: true,
	imports: [CommonModule, CreateEndpointComponent, CreateSubscriptionModule, ButtonComponent, RouterModule, TooltipComponent],
	templateUrl: './create-endpoint.component.html',
	styleUrls: ['./create-endpoint.component.scss']
})
export class CreatePortalEndpointComponent implements OnInit {
	@ViewChild(CreateEndpointComponent) createEndpointForm!: CreateEndpointComponent;
	@ViewChild(CreateSubscriptionComponent) createSubscriptionForm!: CreateSubscriptionComponent;
	@Output('onAction') onAction = new EventEmitter();
	@Input('subscription') subscription?: SUBSCRIPTION;

	isCreatingEndpoint = false;

	constructor(private subscriptionService: CreateSubscriptionService, private endpointService: CreateEndpointService, private generalService: GeneralService) {}

	ngOnInit(): void {}

	async createEndpoint() {
		this.isCreatingEndpoint = true;
		if (this.createEndpointForm.addNewEndpointForm.invalid) return this.createEndpointForm.addNewEndpointForm.markAllAsTouched();

		// check if configs are added, else delete the properties
		const subscriptionData = structuredClone(this.createSubscriptionForm.subscriptionForm.value);
		const retryDuration = this.createSubscriptionForm.subscriptionForm.get('retry_config.duration');
		retryDuration ? (subscriptionData.retry_config.duration = retryDuration?.value + 's') : delete subscriptionData.retry_config;

		const endpointFormValue = structuredClone(this.createEndpointForm.addNewEndpointForm.value);
		delete endpointFormValue.authentication;

		try {
			const endpointDetails = await this.endpointService.addNewEndpoint({ body: endpointFormValue });
			const subscriptionDetails = await this.subscriptionService.createSubscription({ ...this.createSubscriptionForm.subscriptionForm.value, name: `${endpointDetails?.data.title}'s Subscription`, endpoint_id: endpointDetails?.data.uid });
			this.generalService.showNotification({ message: 'Endpint created successfully', style: 'success' });
			this.onAction.emit({ action: 'create', data: subscriptionDetails });
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
			await this.endpointService.editEndpoint({ endpointId: this.subscription?.endpoint_metadata?.uid || '', body: endpointFormValue });
			const subscriptionDetails = await this.subscriptionService.updateSubscription({ data: { ...this.createSubscriptionForm.subscriptionForm.value, endpoint_id: this.subscription?.endpoint_metadata?.uid || '' }, id: this.subscription?.uid || '' });
			this.generalService.showNotification({ message: 'Endpint updated successfully', style: 'success' });
			this.onAction.emit({ action: 'create', data: subscriptionDetails });
			this.isCreatingEndpoint = false;
		} catch (error) {
			this.isCreatingEndpoint = false;
		}
	}
}
