import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { SUBSCRIPTION } from 'src/app/models/subscription';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';

@Component({
	selector: 'app-subscriptions',
	templateUrl: './subscriptions.component.html',
	styleUrls: ['./subscriptions.component.scss']
})
export class SubscriptionsComponent implements OnInit {
	activeSubscription?: SUBSCRIPTION;
	shouldShowCreateSubscriptionModal = false;
	projectId?: string;
	subscriptions?: { content: SUBSCRIPTION[]; pagination?: PAGINATION };
	subscriptionsLoaders = [1, 2, 3, 4, 5];
	isLoadindingSubscriptions = false;
	isDeletingSubscription = false;
	showUpdateSubscriptionModal = false;
	showDeleteSubscriptionModal = false;

	constructor(private route: ActivatedRoute, public privateService: PrivateService, public router: Router, private generalService: GeneralService) {
		this.projectId = this.privateService.activeProjectDetails?.uid;

		const urlParam = route.snapshot.params.id;
		if (urlParam && urlParam === 'new') this.shouldShowCreateSubscriptionModal = true;
		if (urlParam && urlParam !== 'new') this.showUpdateSubscriptionModal = true;
	}

	async ngOnInit() {
		await this.getSubscriptions();
		this.route.queryParams.subscribe(params => (this.activeSubscription = this.subscriptions?.content.find(subscription => subscription.uid === params?.id)));
	}

	async getSubscriptions(requestDetails?: CURSOR) {
		this.isLoadindingSubscriptions = true;

		try {
			const subscriptionsResponse = await this.privateService.getSubscriptions(requestDetails);
			this.subscriptions = subscriptionsResponse.data;
			this.subscriptions?.content?.length === 0 ? localStorage.setItem('isActiveProjectConfigurationComplete', 'false') : localStorage.setItem('isActiveProjectConfigurationComplete', 'true');
			this.isLoadindingSubscriptions = false;
		} catch (error) {
			this.isLoadindingSubscriptions = false;
		}
	}

	closeModal() {
		this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails?.uid + '/subscriptions');
	}

	createSubscription(action: any) {
		this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails?.uid + '/subscriptions');
		if (action !== 'cancel') this.generalService.showNotification({ message: `Subscription has been ${action}d successfully`, style: 'success' });
	}

	async deleteSubscripton() {
		this.isDeletingSubscription = true;

		try {
			const response = await this.privateService.deleteSubscription(this.activeSubscription?.uid || '');
			this.generalService.showNotification({ message: response?.message, style: 'success' });
			this.getSubscriptions();
			delete this.activeSubscription;
			this.showDeleteSubscriptionModal = false;
			this.isDeletingSubscription = false;
		} catch (error) {
			this.isDeletingSubscription = false;
		}
	}

	getEndpointSecret(endpointSecrets: any) {
		return endpointSecrets?.length === 1 ? endpointSecrets[0].value : endpointSecrets[endpointSecrets?.length - 1].value;
	}
}
