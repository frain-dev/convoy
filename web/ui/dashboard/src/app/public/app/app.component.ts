import { Component, OnInit, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { EVENT, EVENT_DELIVERY } from 'src/app/models/event.model';
import { PAGINATION } from 'src/app/models/global.model';
import { SUBSCRIPTION } from 'src/app/models/subscription';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { AppService } from './app.service';
import { CliKeysComponent } from 'src/app/private/pages/project/endpoint-details/cli-keys/cli-keys.component';
import { EndpointDetailsService } from 'src/app/private/pages/project/endpoint-details/endpoint-details.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { PrivateService } from 'src/app/private/private.service';

@Component({
	selector: 'app-app',
	templateUrl: './app.component.html',
	styleUrls: ['./app.component.scss']
})
export class AppComponent implements OnInit {
	@ViewChild('subscriptionDropdown') dropdownComponent!: DropdownComponent;
	@ViewChild(CliKeysComponent) cliKeys!: CliKeysComponent;
	tableHead = ['Name', 'Endpoint', 'Created At', 'Updated At', 'Status', ''];
	token: string = this.route.snapshot.queryParams.token;
	subscriptions!: { content: SUBSCRIPTION[]; pagination: PAGINATION };
	events!: { content: EVENT[]; pagination: PAGINATION };
	eventDeliveries!: { content: EVENT_DELIVERY[]; pagination: PAGINATION };
	activeSubscription?: SUBSCRIPTION;
	eventDeliveryFilteredByEventId!: string;
	isloadingSubscriptions = false;
	showCreateSubscriptionModal = false;
	showDeleteSubscriptionModal = false;
	isDeletingSubscription = false;
	subscriptionId = this.route.snapshot.params.id;
	showCreateSubscription = false;
	showSubscriptionError = false;
	showEndpointSecret: boolean = false;
	showCreateEndpoint = false;
	isTogglingEndpoint = false;

	constructor(private appService: AppService, private route: ActivatedRoute, private endpointDetailsService: EndpointDetailsService, private generalService: GeneralService, private endpointService: EndpointDetailsService) {}

	ngOnInit(): void {
		this.getSubscripions();
		if (this.route.snapshot.queryParams?.createSub) localStorage.setItem('CONVOY_APP__SHOW_CREATE_SUB', this.route.snapshot.queryParams?.createSub);
		const subscribeButtonState = localStorage.getItem('CONVOY_APP__SHOW_CREATE_SUB');

		switch (subscribeButtonState) {
			case 'true':
				this.showCreateSubscription = true;
				this.tableHead.pop();
				break;
			case 'false':
				this.showCreateSubscription = false;
				break;

			default:
				this.showCreateSubscription = false;
				break;
		}
	}

	async getSubscripions() {
		this.isloadingSubscriptions = true;
		try {
			const subscriptions = await this.appService.getSubscriptions();
			this.subscriptions = subscriptions.data;
			this.isloadingSubscriptions = false;
			this.showCreateSubscriptionModal = false;
			this.showSubscriptionError = false;
		} catch (_error) {
			this.showSubscriptionError = true;
			this.isloadingSubscriptions = false;
		}
	}

	getEventDeliveries(eventId: string) {
		this.eventDeliveryFilteredByEventId = eventId;
	}

	async deleteSubscription() {
		if (!this.activeSubscription) return;

		this.isDeletingSubscription = true;
		try {
			await this.appService.deleteSubscription(this.activeSubscription.uid);
			await this.endpointDetailsService.deleteEndpoint(this.activeSubscription.endpoint_metadata?.uid || '');
			this.getSubscripions();
			this.isDeletingSubscription = false;
			this.showDeleteSubscriptionModal = false;
			delete this.activeSubscription;
		} catch (error) {
			this.isDeletingSubscription = false;
		}
	}

	hasFilter(filterObject: { headers: Object; body: Object }): boolean {
		return Object.keys(filterObject.body).length > 0 || Object.keys(filterObject.headers).length > 0;
	}

	async sendTestEvent(endpointId?: string) {
		if (!endpointId) return;

		const testEvent = {
			data: { data: 'Test event from Convoy', convoy: 'https://github.com/frain-dev/convoy' },
			endpoint_id: endpointId,
			event_type: 'test.convoy'
		};

		try {
			const response = await this.endpointDetailsService.sendEvent({ body: testEvent });
			this.generalService.showNotification({ message: response.message, style: 'success' });
		} catch (error) {
			console.log(error);
		}
	}

	async toggleEndpoint(subscriptionIndex: number, endpointDetails?: ENDPOINT) {
		if (!endpointDetails?.uid) return;
		this.isTogglingEndpoint = true;

		try {
			const response = await this.endpointDetailsService.toggleEndpoint(endpointDetails?.uid);
			this.subscriptions.content[subscriptionIndex].endpoint_metadata = response.data;
			this.generalService.showNotification({ message: `${endpointDetails?.title} status updated successfully`, style: 'success' });
			this.isTogglingEndpoint = false;
		} catch {
			this.isTogglingEndpoint = false;
		}
	}

	hideSubscriptionDropdown() {
		this.dropdownComponent.show = false;
	}
}
