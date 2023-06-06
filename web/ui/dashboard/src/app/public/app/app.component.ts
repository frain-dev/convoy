import { Component, OnInit, ViewChild } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { EVENT_DELIVERY } from 'src/app/models/event.model';
import { PAGINATION } from 'src/app/models/global.model';
import { SUBSCRIPTION } from 'src/app/models/subscription';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { AppService } from './app.service';
import { EndpointDetailsService } from 'src/app/private/pages/project/endpoint-details/endpoint-details.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { ENDPOINT, PORTAL_LINK } from 'src/app/models/endpoint.model';

@Component({
	selector: 'app-app',
	templateUrl: './app.component.html',
	styleUrls: ['./app.component.scss']
})
export class AppComponent implements OnInit {
	@ViewChild('subscriptionDropdown') dropdownComponent!: DropdownComponent;
	token: string = this.route.snapshot.queryParams.token;
	subscriptions!: { content: SUBSCRIPTION[]; pagination: PAGINATION };
	eventDeliveries!: { content: EVENT_DELIVERY[]; pagination: PAGINATION };
	activeSubscription?: SUBSCRIPTION;
	eventDeliveryFilteredByEventId!: string;
	isloadingSubscriptions = false;
	showEndpointSecret: boolean = false;
	showCreateEndpoint = false;
	isTogglingEndpoint = false;
	portalDetails!: PORTAL_LINK;

	constructor(private appService: AppService, private route: ActivatedRoute, private endpointDetailsService: EndpointDetailsService, private generalService: GeneralService) {}

	ngOnInit(): void {
		Promise.all([this.getSubscripions(), this.getPortalDetails()]);
	}

	async getPortalDetails() {
		try {
			const portalLinkDetails = await this.appService.getPortalDetail();
			this.portalDetails = portalLinkDetails.data;
		} catch (_error) {}
	}

	async getSubscripions() {
		this.isloadingSubscriptions = true;
		try {
			const subscriptions = await this.appService.getSubscriptions();
			this.subscriptions = subscriptions.data;
			this.isloadingSubscriptions = false;
		} catch (_error) {
			this.isloadingSubscriptions = false;
		}
	}

	getEventDeliveries(eventId: string) {
		this.eventDeliveryFilteredByEventId = eventId;
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
