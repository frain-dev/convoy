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
import { CreateEndpointComponent } from 'src/app/private/components/create-endpoint/create-endpoint.component';
import { PaginationComponent } from 'src/app/private/components/pagination/pagination.component';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { FormsModule } from '@angular/forms';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';

interface PORTAL_ENDPOINT extends ENDPOINT {
	subscription?: SUBSCRIPTION;
}
@Component({
	selector: 'convoy-endpoints',
	standalone: true,
	imports: [CommonModule, DialogDirective, EndpointSecretComponent, TagComponent, StatusColorModule, CardComponent, DropdownComponent, DropdownOptionDirective, ButtonComponent, CreateEndpointComponent, PaginationComponent, FormsModule, CopyButtonComponent],
	templateUrl: './endpoints.component.html',
	styleUrls: ['./endpoints.component.scss']
})
export class EndpointsComponent implements OnInit {
	@ViewChild('subscriptionDropdown') dropdownComponent!: DropdownComponent;
	token: string = this.route.snapshot.queryParams.token;
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

	ngOnInit(): void {
		Promise.all([this.getPortalDetails(), this.getEndpoints()]).then(() => {
			this.activeEndpoint = this.endpoints.find(endpoint => endpoint.uid === this.route.snapshot.queryParams.endpointId);
			this.showCreateEndpoint = !!this.route.snapshot.queryParams.endpointId;
		});
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
		this.router.navigate(['/portal/subscriptions'], { queryParams: { token: this.token, endpointId: this.activeEndpoint?.uid } });
	}

	hideSubscriptionDropdown() {
		this.dropdownComponent.show = false;
	}

	openEndpointForm(action: 'create' | 'update') {
		this.action = action;
		this.showCreateEndpoint = true;
		this.location.go(`/portal/endpoints/${action === 'create' ? 'new' : this.activeEndpoint?.uid}?token=${this.token}`);
		document.getElementsByTagName('body')[0].classList.add('overflow-hidden');
	}

	onCloseEndpointForm() {
		this.activeEndpoint = undefined;
		this.getEndpoints();
		this.showCreateEndpoint = false;
		this.location.go('/portal/endpoints?token=' + this.token);
	}

	goBack(isForm?: boolean) {
		if (isForm) this.showCreateEndpoint = false;
		this.activeEndpoint = undefined;
		this.getEndpoints();
		this.location.back();
		document.getElementsByTagName('body')[0].classList.remove('overflow-hidden');
	}
}
