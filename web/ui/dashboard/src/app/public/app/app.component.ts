import { Component, OnInit } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { EVENT, EVENT_DELIVERY } from 'src/app/models/event.model';
import { PAGINATION } from 'src/app/models/global.model';
import { SUBSCRIPTION } from 'src/app/models/subscription';
import { AppService } from './app.service';

type PAGE_TABS = 'events' | 'event deliveries';

@Component({
	selector: 'app-app',
	templateUrl: './app.component.html',
	styleUrls: ['./app.component.scss']
})
export class AppComponent implements OnInit {
	tableHead = ['Name', 'Endpoint', 'Created At', 'Updated At', 'Event Types', 'Status', ''];
	token: string = this.route.snapshot.params.token;
	subscriptions!: { content: SUBSCRIPTION[]; pagination: PAGINATION };
	tabs: ['events', 'event deliveries'] = ['events', 'event deliveries'];
	activeTab: PAGE_TABS = 'events';
	events!: { content: EVENT[]; pagination: PAGINATION };
	eventDeliveries!: { content: EVENT_DELIVERY[]; pagination: PAGINATION };
	eventDeliveryFilteredByEventId!: string;
	isloadingSubscriptions = false;
	showCreateSubscriptionModal = false;
	subscriptionId = this.route.snapshot.params.id;

	constructor(private appService: AppService, private route: ActivatedRoute) {}

	ngOnInit(): void {
		this.getSubscripions();
	}

	async getSubscripions() {
		this.isloadingSubscriptions = true;
		try {
			const subscriptions = await this.appService.getSubscriptions(this.token);
			this.subscriptions = subscriptions.data;
			this.isloadingSubscriptions = false;
			this.showCreateSubscriptionModal = false;
		} catch (_error) {
			this.isloadingSubscriptions = false;
		}
	}

	toggleActiveTab(tab: PAGE_TABS) {
		this.activeTab = tab;
	}

	getEventDeliveries(eventId: string) {
		this.eventDeliveryFilteredByEventId = eventId;
		this.toggleActiveTab('event deliveries');
	}

	async deleteSubscription(subscriptionId: string) {
		try {
			await this.appService.deleteSubscription(this.token, subscriptionId);
			this.getSubscripions();
		} catch (error) {
			console.log(error);
		}
	}
}
