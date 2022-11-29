import { Component, OnInit, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { EVENT, EVENT_DELIVERY } from 'src/app/models/event.model';
import { PAGINATION } from 'src/app/models/global.model';
import { SUBSCRIPTION } from 'src/app/models/subscription';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { AppService } from './app.service';
import { CliKeysComponent } from 'src/app/private/pages/project/apps/app-details/cli-keys/cli-keys.component';

type EVENT_PAGE_TABS = 'event deliveries';

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
	subscriptions!: { content: SUBSCRIPTION[]; pagination?: PAGINATION };
	eventTabs: ['event deliveries'] = ['event deliveries'];
	tabs: string[] = ['subscriptions'];
	activeEventsTab: EVENT_PAGE_TABS = 'event deliveries';
	activeTab: string = 'subscriptions';
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
	showCliKeysAndDevices = false;
	showSubscriptionError = false;
	showCliError = false;
	isCliAvailable: boolean = false;
	subscriptionIdsString = '?';
	subscriptionIds = this.route.snapshot.queryParams?.subscriptionID || [];

	constructor(private appService: AppService, private route: ActivatedRoute, private router: Router) {
		// for subscription portal use
		if (typeof this.subscriptionIds === 'string') this.subscriptionIds = [this.subscriptionIds];
		if (this.subscriptionIds.length > 0) this.subscriptionIds?.forEach((id: string) => (this.subscriptionIdsString += 'subscriptionID=' + id + '&'));
	}

	ngOnInit() {
		if (this.subscriptionIds.length > 0) {
			this.subscriptions = { content: [] };
			this.subscriptionIds.forEach(async (id: string) => this.subscriptions?.content.push(await this.getSubscripion(id)));
		} else {
			this.getSubscripions();
		}
		this.checkFlags();

		if (this.route.snapshot.queryParams?.showCli) localStorage.setItem('CONVOY_APP__SHOW_CLI', this.route.snapshot.queryParams?.showCli);
		if (this.route.snapshot.queryParams?.createSub) localStorage.setItem('CONVOY_APP__SHOW_CREATE_SUB', this.route.snapshot.queryParams?.createSub);

		const subscribeButtonState = localStorage.getItem('CONVOY_APP__SHOW_CREATE_SUB');
		subscribeButtonState ? (this.showCreateSubscription = JSON.parse(subscribeButtonState)) : (this.showCreateSubscription = false);

		const showCliKeysAndDevices = localStorage.getItem('CONVOY_APP__SHOW_CLI');
		showCliKeysAndDevices ? (this.showCliKeysAndDevices = JSON.parse(showCliKeysAndDevices)) : (this.showCliKeysAndDevices = false);
	}

	async checkFlags() {
		this.isCliAvailable = await this.appService.getFlag('can_create_cli_api_key', this.token);
		if (this.isCliAvailable && this.showCliKeysAndDevices) this.tabs.push('cli keys', 'devices');
	}

	async getSubscripion(subscriptionId: string): Promise<SUBSCRIPTION> {
		return new Promise(async (resolve, reject) => {
			try {
				const subscription = await this.appService.getSubscription(this.token, subscriptionId);
				return resolve(subscription.data);
			} catch (error) {
				return reject(error);
			}
		});
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

	toggleActiveTab(tab: string) {
		this.activeTab = tab;
	}

	toggleEventsTab(tab: EVENT_PAGE_TABS) {
		this.activeEventsTab = tab;
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

	onCreateSubscription() {
		this.router.navigate(['/app', this.token]);
	}
}
