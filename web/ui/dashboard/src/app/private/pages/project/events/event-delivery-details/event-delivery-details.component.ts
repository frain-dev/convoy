import { Component, EventEmitter, HostListener, OnInit, Output } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { EVENT_DELIVERY, EVENT_DELIVERY_ATTEMPT } from 'src/app/models/event.model';
import { EventDeliveryDetailsService } from './event-delivery-details.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { EventsService } from '../events.service';
import { PrivateService } from 'src/app/private/private.service';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { EndpointsService } from '../../endpoints/endpoints.service';

@Component({
	selector: 'app-event-delivery-details',
	templateUrl: './event-delivery-details.component.html',
	styleUrls: ['./event-delivery-details.component.scss']
})
export class EventDeliveryDetailsComponent implements OnInit {
	@Output('onViewEndpoint') onViewEndpoint = new EventEmitter<any>();
	eventDelsDetails?: EVENT_DELIVERY;
	eventDeliveryAtempt?: EVENT_DELIVERY_ATTEMPT;
	eventDeliveryAtempts: EVENT_DELIVERY_ATTEMPT[] = [];
	selectedDeliveryAttempt?: EVENT_DELIVERY_ATTEMPT;
	isLoadingDeliveryDetails = false;
	isloadingDeliveryAttempts = false;
	isloadingEndpoint = false;
	shouldRenderSmallSize = false;
	eventDeliveryId = this.route.snapshot.params?.id;
	screenWidth = window.innerWidth;
	portalToken = this.route.snapshot.queryParams?.token;

	constructor(
		private route: ActivatedRoute,
		private router: Router,
		private privateService: PrivateService,
		private eventDeliveryDetailsService: EventDeliveryDetailsService,
		public generalService: GeneralService,
		private eventsService: EventsService,
		private endpointService: EndpointsService
	) {}

	ngOnInit(): void {
		const eventDeliveryId = this.route.snapshot.params.id;
		this.getEventDeliveryDetails(eventDeliveryId).then(_ => this.getEndpoint());
		this.getEventDeliveryAttempts(eventDeliveryId);
	}

	async getEventDeliveryDetails(id: string) {
		this.isLoadingDeliveryDetails = true;

		try {
			const response = await this.eventDeliveryDetailsService.getEventDeliveryDetails(id);
			this.eventDelsDetails = response.data;
			this.isLoadingDeliveryDetails = false;
		} catch (error) {
			this.isLoadingDeliveryDetails = false;
		}
	}

	async forceRetryEvent(requestDetails: { e: any; eventDeliveryId: string }) {
		const payload = {
			ids: [requestDetails.eventDeliveryId]
		};

		try {
			await this.eventsService.forceRetryEvent({ body: payload });
			this.getEventDeliveryDetails(requestDetails.eventDeliveryId);
			this.generalService.showNotification({ message: 'Force Retry Request Sent', style: 'success' });
		} catch (error: any) {
			this.generalService.showNotification({ message: `${error?.error?.message ? error?.error?.message : 'An error occured'}`, style: 'error' });
			return error;
		}
	}

	async retryEvent(requestDetails: { e: any; eventDeliveryId: string }) {
		try {
			await this.eventsService.retryEvent({ eventId: requestDetails.eventDeliveryId });
			this.getEventDeliveryDetails(requestDetails.eventDeliveryId);
			this.generalService.showNotification({ message: 'Retry Request Sent', style: 'success' });
		} catch (error: any) {
			this.generalService.showNotification({ message: `${error?.error?.message ? error?.error?.message : 'An error occured'}`, style: 'error' });
			return error;
		}
	}

	async getEventDeliveryAttempts(eventId: string) {
		this.isloadingDeliveryAttempts = true;

		try {
			const response = await this.eventDeliveryDetailsService.getEventDeliveryAttempts({ eventId });
			const deliveries = response.data;
			this.eventDeliveryAtempts = deliveries.reverse();
			this.eventDeliveryAtempt = this.eventDeliveryAtempts[this.eventDeliveryAtempts.length - 1];
			this.selectedDeliveryAttempt = this.eventDeliveryAtempt;

			this.isloadingDeliveryAttempts = false;
		} catch (error) {
			this.isloadingDeliveryAttempts = false;
		}
	}

	viewEndpoint(endpointId: string) {
		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid + '/endpoints/' + endpointId);
	}

	checkScreenSize() {
		this.screenWidth > 1010 ? (this.shouldRenderSmallSize = false) : (this.shouldRenderSmallSize = true);
	}

	@HostListener('window:resize', ['$event'])
	onWindowResize() {
		this.screenWidth = window.innerWidth;
		this.checkScreenSize();
	}

	async getEndpoint() {
		if (!this.eventDelsDetails?.endpoint_id) return;
		this.isloadingEndpoint = true;

		try {
			const response = await this.endpointService.getEndpoint(this.eventDelsDetails?.endpoint_id);
			this.eventDelsDetails.endpoint_metadata = response.data;

			this.isloadingEndpoint = false;
		} catch (error) {
			this.isloadingEndpoint = false;
		}
	}
}
