import { Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { CreateEndpointSubscriptionComponent } from '../components/create-endpoint-subscription/create-endpoint-subscription.component';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'app-example-create-endpoint-subscription',
	standalone: true,
	imports: [CommonModule, CreateEndpointSubscriptionComponent],
	template: `
		<div class="container mx-auto py-8">
			<h1 class="text-2xl font-bold mb-6">Create New Endpoint with Subscription</h1>

			<convoy-create-endpoint-subscription [showAction]="'true'" (onAction)="handleEndpointAction($event)"></convoy-create-endpoint-subscription>
		</div>
	`,
	styles: []
})
export class ExampleCreateEndpointSubscriptionComponent {
	constructor(private generalService: GeneralService) {}

	handleEndpointAction(event: any) {
		console.log('Endpoint action:', event);

		if (event.action === 'close') {
			// Handle close action
			this.generalService.showNotification({
				message: 'Endpoint creation cancelled',
				style: 'warning'
			});
		} else if (event.action === 'save') {
			// Handle save action
			this.generalService.showNotification({
				message: `Endpoint "${event.data.name}" created successfully with subscription`,
				style: 'success'
			});
		} else if (event.action === 'update') {
			// Handle update action
			this.generalService.showNotification({
				message: `Endpoint "${event.data.name}" updated successfully`,
				style: 'success'
			});
		}
	}
}
