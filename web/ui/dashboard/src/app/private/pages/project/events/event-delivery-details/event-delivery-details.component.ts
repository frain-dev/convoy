import { AfterViewInit, Component, ElementRef, EventEmitter, HostListener, OnInit, Output, ViewChild } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { EVENT_DELIVERY, EVENT_DELIVERY_ATTEMPT } from 'src/app/models/event.model';
import { EventDeliveryDetailsService } from './event-delivery-details.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { EventsService } from '../events.service';

@Component({
    selector: 'app-event-delivery-details',
    templateUrl: './event-delivery-details.component.html',
    styleUrls: ['./event-delivery-details.component.scss'],
    standalone: false
})
export class EventDeliveryDetailsComponent implements OnInit, AfterViewInit {
	@Output('onViewEndpoint') onViewEndpoint = new EventEmitter<any>();
	@ViewChild('metaStrip') metaStrip?: ElementRef<HTMLDivElement>;
	canScrollLeft = false;
	canScrollRight = false;
	private readonly stripFadeWidth = 64;
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
		private eventDeliveryDetailsService: EventDeliveryDetailsService,
		public generalService: GeneralService,
		private eventsService: EventsService
	) {}

	ngOnInit(): void {
		const eventDeliveryId = this.route.snapshot.params.id;
		this.getEventDeliveryDetails(eventDeliveryId);
		this.getEventDeliveryAttempts(eventDeliveryId);
	}

	ngAfterViewInit(): void {
		this.updateStripFade();
	}

	async getEventDeliveryDetails(id: string) {
		this.isLoadingDeliveryDetails = true;

		try {
			const response = await this.eventDeliveryDetailsService.getEventDeliveryDetails(id);
			this.eventDelsDetails = response.data;
			this.isLoadingDeliveryDetails = false;
			this.scheduleStripFadeUpdate();
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
			// API returns attempts oldest -> newest; reverse so the strip renders newest first.
			this.eventDeliveryAtempts = deliveries.reverse();
			// Newest attempt is now at index 0, so LATEST ATTEMPT must read the head, not the tail.
			this.eventDeliveryAtempt = this.eventDeliveryAtempts[0];
			this.selectedDeliveryAttempt = this.eventDeliveryAtempt;

			this.isloadingDeliveryAttempts = false;
			// Attempt data fills LATEST ATTEMPT / IP ADDRESS / API VERSION, so the strip width
			// can change after this second response; recompute the scroll arrows and fade.
			this.scheduleStripFadeUpdate();
		} catch (error) {
			this.isloadingDeliveryAttempts = false;
		}
	}

	formatLatencySeconds(latencySeconds?: number, deliveryStatus?: string): string {
		if (deliveryStatus !== 'Success' || latencySeconds === undefined || latencySeconds === null) return '-';

		if (latencySeconds < 1) {
			return `${Math.round(latencySeconds * 1000)}ms`;
		}

		return `${latencySeconds.toFixed(2)}s`;
	}

	formatPreciseTimestamp(value?: string | Date): string {
		if (!value) return '';

		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return String(value);

		return date.toISOString();
	}

	// Render an inline timestamp with milliseconds, following the viewer's browser
	// locale. The locale (not a hardcoded pattern) decides 12h vs 24h, so 24h locales
	// like fr-FR drop the AM/PM marker while en-US keeps it. The hover/copy value
	// (formatPreciseTimestamp) stays UTC ISO.
	formatLocalTimestamp(value?: string | Date): string {
		if (!value) return '-';

		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return String(value);

		return new Intl.DateTimeFormat(undefined, {
			year: 'numeric',
			month: 'short',
			day: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit',
			fractionalSecondDigits: 3
		}).format(date);
	}

	isEndpointDeleted(): boolean {
		return !!this.eventDelsDetails?.endpoint_metadata?.deleted_at;
	}

	checkScreenSize() {
		this.screenWidth > 1010 ? (this.shouldRenderSmallSize = false) : (this.shouldRenderSmallSize = true);
	}

	onStripScroll() {
		this.updateStripFade();
	}

	scrollStrip(direction: 'left' | 'right') {
		const el = this.metaStrip?.nativeElement;
		if (!el) return;

		const amount = Math.max(el.clientWidth * 0.8, 200);
		el.scrollBy({ left: direction === 'right' ? amount : -amount, behavior: 'smooth' });
	}

	private scheduleStripFadeUpdate() {
		// Recompute once the strip has rendered with the loaded data.
		setTimeout(() => this.updateStripFade());
	}

	updateStripFade() {
		const el = this.metaStrip?.nativeElement;
		if (!el) {
			this.canScrollLeft = false;
			this.canScrollRight = false;
			return;
		}

		const maxScroll = el.scrollWidth - el.clientWidth;
		this.canScrollLeft = el.scrollLeft > 1;
		this.canScrollRight = el.scrollLeft < maxScroll - 1;
	}

	// Android-style fading edge: only fade the side that still has content to scroll to,
	// and leave both edges crisp when the strip fits without scrolling.
	get stripMaskImage(): string {
		const fade = this.stripFadeWidth;
		const leftStop = this.canScrollLeft ? 'transparent 0' : '#000 0';
		const rightStop = this.canScrollRight ? 'transparent 100%' : '#000 100%';
		return `linear-gradient(to right, ${leftStop}, #000 ${fade}px, #000 calc(100% - ${fade}px), ${rightStop})`;
	}

	@HostListener('window:resize')
	onWindowResize() {
		this.screenWidth = window.innerWidth;
		this.checkScreenSize();
		this.updateStripFade();
	}
}
