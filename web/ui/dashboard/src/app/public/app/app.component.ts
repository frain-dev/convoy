import { Component, OnInit, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { EVENT, EVENT_DELIVERY } from 'src/app/models/event.model';
import { PAGINATION } from 'src/app/models/global.model';
import { SUBSCRIPTION } from 'src/app/models/subscription';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { AppService } from './app.service';
import { CliKeysComponent } from 'src/app/private/pages/project/apps/app-details/cli-keys/cli-keys.component';

type EVENT_PAGE_TABS = 'events' | 'event deliveries';
type PAGE_TABS = 'subscriptions' | 'cli keys';

@Component({
	selector: 'app-app',
	templateUrl: './app.component.html',
	styleUrls: ['./app.component.scss']
})
export class AppComponent implements OnInit {
	@ViewChild(DropdownComponent) dropdownComponent!: DropdownComponent;
	@ViewChild(CliKeysComponent) cliKeys!: CliKeysComponent;
	tableHead = ['Name', 'Endpoint', 'Created At', 'Updated At', 'Event Types', 'Status', ''];
	token: string = this.route.snapshot.params.token;
	subscriptions!: { content: SUBSCRIPTION[]; pagination: PAGINATION };
	eventTabs: ['events', 'event deliveries'] = ['events', 'event deliveries'];
	tabs: ['subscriptions', 'cli keys'] = ['subscriptions', 'cli keys'];
	activeEventsTab: EVENT_PAGE_TABS = 'events';
	activeTab: PAGE_TABS = 'subscriptions';
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

	constructor(private appService: AppService, private route: ActivatedRoute, private router: Router) {}

	ngOnInit(): void {
		this.getSubscripions();

		if (this.route.snapshot.queryParams?.createSub) localStorage.setItem('CONVOY_APP__SHOW_CREATE_SUB', this.route.snapshot.queryParams?.createSub);
		const subscribeButtonState = localStorage.getItem('CONVOY_APP__SHOW_CREATE_SUB');

		switch (subscribeButtonState) {
			case 'true':
				this.showCreateSubscription = true;
				break;
			case 'false':
				this.showCreateSubscription = false;
				break;

			default:
				this.showCreateSubscription = true;
				break;
		}
	}

	async getSubscripions() {
		this.isloadingSubscriptions = true;
		try {
			const subscriptions = await this.appService.getSubscriptions(this.token);
			this.subscriptions = subscriptions.data;
			this.isloadingSubscriptions = false;
			this.showCreateSubscriptionModal = false;
			this.showSubscriptionError = false;
		} catch (_error) {
			this.showSubscriptionError = true;
			this.isloadingSubscriptions = false;
		}
	}

	toggleActiveTab(tab: PAGE_TABS) {
		this.activeTab = tab;
	}

	toggleEventsTab(tab: EVENT_PAGE_TABS) {
		this.activeEventsTab = tab;
	}

	getEventDeliveries(eventId: string) {
		this.eventDeliveryFilteredByEventId = eventId;
		this.toggleEventsTab('event deliveries');
	}

	async deleteSubscription() {
		this.isDeletingSubscription = true;
		try {
			await this.appService.deleteSubscription(this.token, this.activeSubscription?.uid || '');
			this.getSubscripions();
			this.isDeletingSubscription = false;
		} catch (error) {
			this.isDeletingSubscription = false;
		}
	}

	closeCreateSubscriptionModal() {
		this.showCreateSubscriptionModal = false;
		this.router.navigate(['/app', this.token]);
	}
}
