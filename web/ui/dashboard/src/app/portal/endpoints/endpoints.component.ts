import { Component, OnInit, ViewChild } from '@angular/core';
import { CommonModule, Location } from '@angular/common';
import { ActivatedRoute, Router } from '@angular/router';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { ENDPOINT, PORTAL_LINK } from 'src/app/models/endpoint.model';
import { SUBSCRIPTION } from 'src/app/models/subscription';
import { GeneralService } from 'src/app/services/general/general.service';
import { EndpointsService } from 'src/app/private/pages/project/endpoints/endpoints.service';
import { DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { EndpointSecretComponent } from 'src/app/private/pages/project/endpoints/endpoint-secret/endpoint-secret.component';
import { PortalService } from '../portal.service';
import { PrivateService } from 'src/app/private/private.service';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { PaginationComponent } from 'src/app/private/components/pagination/pagination.component';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { ControlContainer, FormsModule, FormGroupDirective, ReactiveFormsModule } from '@angular/forms';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { CreatePortalEndpointComponent } from '../create-portal-endpoint/create-portal-endpoint.component';
import { CreateEndpointComponent } from '../../private/components/create-endpoint/create-endpoint.component';

interface PORTAL_ENDPOINT extends ENDPOINT {
	subscription?: SUBSCRIPTION;
}
@Component({
	selector: 'convoy-endpoints',
	standalone: true,
	imports: [
		CommonModule,
		DialogDirective,
		EndpointSecretComponent,
		TagComponent,
		StatusColorModule,
		CardComponent,
		DropdownComponent,
		DropdownOptionDirective,
		ButtonComponent,
		CreatePortalEndpointComponent,
		PaginationComponent,
		FormsModule,
		ReactiveFormsModule,
		CopyButtonComponent,
		CreateEndpointComponent
	],
	providers: [{ provide: ControlContainer, useValue: null }, FormGroupDirective],
	templateUrl: './endpoints.component.html',
	styleUrls: ['./endpoints.component.scss']
})
export class EndpointsComponent implements OnInit {
	@ViewChild('subscriptionDropdown') dropdownComponent!: DropdownComponent;
	token: string = this.route.snapshot.queryParams.token;
	ownerId: string = this.route.snapshot.queryParams.owner_id;
	currentRoute = window.location.pathname.split('/').reverse()[0];
	activeEndpoint?: PORTAL_ENDPOINT;
	eventDeliveryFilteredByEventId!: string;
	isloadingSubscriptions = false;
	showCreateEndpoint = false;
	showSubscriptionsList = false;
	isTogglingEndpoint = false;
	portalDetails!: PORTAL_LINK;
	fetchedEndpoints?: { content: ENDPOINT[]; pagination?: PAGINATION };
	displayedEndpoints?: { date: string; content: ENDPOINT[] }[];
	endpoints: PORTAL_ENDPOINT[] = [];
	action: 'create' | 'update' = 'create';
	endpointSearchString = '';

	constructor(private route: ActivatedRoute, private generalService: GeneralService, private endpointService: EndpointsService, private portalService: PortalService, private privateService: PrivateService, private location: Location, private router: Router) {}

	async ngOnInit(): Promise<void> {
		await Promise.all([this.getPortalDetails(), this.getEndpoints()]);

		// Check if we have an endpoint ID in the route
		const endpointId = this.route.snapshot.params['id'];
		if (endpointId) {
			// Find the endpoint in the list
			this.activeEndpoint = this.endpoints.find(endpoint => endpoint.uid === endpointId);

			// If we found the endpoint or if it's a new endpoint, show the form
			if (this.activeEndpoint || endpointId === 'new') {
				this.action = endpointId === 'new' ? 'create' : 'update';
				this.showCreateEndpoint = true;
				document.getElementsByTagName('body')[0].classList.add('overflow-hidden');
			}
		} else {
			// Check query params for endpointId
			const queryEndpointId = this.route.snapshot.queryParams.endpointId;
			if (queryEndpointId) {
				this.activeEndpoint = this.endpoints.find(endpoint => endpoint.uid === queryEndpointId);
				this.showCreateEndpoint = !!queryEndpointId;
			}
		}
	}

	async getPortalDetails() {
		try {
			const portalLinkDetails = await this.portalService.getPortalDetail();
			this.portalDetails = portalLinkDetails.data;
		} catch (_error) {}
	}

	async getEndpoints(requestDetails?: CURSOR & { q?: string }) {
		this.isloadingSubscriptions = true;
		try {
			const endpoints = await this.privateService.getEndpoints(requestDetails);
			this.fetchedEndpoints = endpoints.data;
			this.endpoints = endpoints.data.content;
			this.displayedEndpoints = this.generalService.setContentDisplayed(endpoints.data.content, 'desc');

			this.isloadingSubscriptions = false;
		} catch (_error) {
			this.isloadingSubscriptions = false;
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
			const response = await this.endpointService.sendEvent({ body: testEvent });
			this.generalService.showNotification({ message: response.message, style: 'success' });
		} catch (error) {
			console.log(error);
		}
	}

	async toggleEndpoint(subscriptionIndex: number, endpointDetails?: ENDPOINT) {
		if (!endpointDetails?.uid) return;
		this.isTogglingEndpoint = true;

		try {
			const response = await this.endpointService.toggleEndpoint(endpointDetails?.uid);
			this.endpoints[subscriptionIndex] = { ...this.endpoints[subscriptionIndex], ...response.data };
			this.generalService.showNotification({ message: `${endpointDetails?.name || endpointDetails?.title} status updated successfully`, style: 'success' });
			this.isTogglingEndpoint = false;
		} catch {
			this.isTogglingEndpoint = false;
		}
	}

	goToSubscriptionsPage(endpoint: ENDPOINT) {
		this.activeEndpoint = endpoint;
		this.showSubscriptionsList = true;
		const queryParams: any = { endpointId: this.activeEndpoint?.uid };
		if (this.token) queryParams.token = this.token;
		if (this.ownerId) queryParams.owner_id = this.ownerId;

		this.router.navigate(['/portal/subscriptions'], { queryParams });
	}

	hideSubscriptionDropdown() {
		this.dropdownComponent.show = false;
	}

	openEndpointForm(action: 'create' | 'update') {
		this.action = action;
		this.showCreateEndpoint = true;
		const queryParams = new URLSearchParams();
		if (this.token) queryParams.append('token', this.token);
		if (this.route.snapshot.queryParams.owner_id) queryParams.append('owner_id', this.route.snapshot.queryParams.owner_id);

		this.location.go(`/portal/endpoints/${action === 'create' ? 'new' : this.activeEndpoint?.uid}?${queryParams.toString()}`);
		document.getElementsByTagName('body')[0].classList.add('overflow-hidden');
	}

	onCloseEndpointForm() {
		this.activeEndpoint = undefined;
		this.getEndpoints();
		this.showCreateEndpoint = false;

		const queryParams = new URLSearchParams();
		if (this.token) queryParams.append('token', this.token);
		if (this.route.snapshot.queryParams.owner_id) queryParams.append('owner_id', this.route.snapshot.queryParams.owner_id);

		this.location.go(`/portal/endpoints?${queryParams.toString()}`);
	}

	goBack(isForm?: boolean) {
		if (isForm) this.showCreateEndpoint = false;
		this.activeEndpoint = undefined;
		this.getEndpoints();
		this.location.back();
		document.getElementsByTagName('body')[0].classList.remove('overflow-hidden');
	}
}
