import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { PAGINATION } from 'src/app/models/global.model';
import { SUBSCRIPTION } from 'src/app/models/subscription';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { SubscriptionsService } from './subscriptions.service';

@Component({
	selector: 'app-subscriptions',
	templateUrl: './subscriptions.component.html',
	styleUrls: ['./subscriptions.component.scss']
})
export class SubscriptionsComponent implements OnInit {
	activeSubscription?: SUBSCRIPTION;
	shouldShowCreateSubscriptionModal = false;
	projectId!: string;
	subscriptions!: { content: SUBSCRIPTION[]; pagination: PAGINATION };
	subscriptionsLoaders = [1, 2, 3, 4, 5];
	isLoadindingSubscriptions = false;
	isDeletingSubscription = false;
	showUpdateSubscriptionModal = false;
	textCopied = false;
	showDeleteSubscriptionModal = false;

	constructor(private route: ActivatedRoute, public privateService: PrivateService, private router: Router, private subscriptionsService: SubscriptionsService, private generalService: GeneralService) {
		this.route.queryParams.subscribe(params => (this.activeSubscription = this.subscriptions?.content.find(source => source.uid === params?.id)));
		this.projectId = this.privateService.activeProjectDetails.uid;

		const urlParam = route.snapshot.params.id;
		if (urlParam && urlParam === 'new') this.shouldShowCreateSubscriptionModal = true;
		if (urlParam && urlParam !== 'new') this.showUpdateSubscriptionModal = true;
	}

	async ngOnInit() {
		await this.getSubscriptions();
	}

	async getSubscriptions(requestDetails?: { page?: number }) {
		this.isLoadindingSubscriptions = true;

		try {
			const subscriptionsResponse = await this.subscriptionsService.getSubscriptions({ page: requestDetails?.page });
			this.subscriptions = subscriptionsResponse.data;
			this.isLoadindingSubscriptions = false;
		} catch (error) {
			this.isLoadindingSubscriptions = false;
		}
	}

	closeModal() {
		this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails.uid + '/subscriptions');
	}

	createSubscription(action: any) {
		this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails.uid + '/subscriptions');
		if (action !== 'cancel') this.generalService.showNotification({ message: 'Subscription has been created successfully', style: 'success' });
	}

	copyText(text?: string) {
		if (!text) return;
		const el = document.createElement('textarea');
		el.value = text;
		document.body.appendChild(el);
		el.select();
		document.execCommand('copy');
		this.generalService.showNotification({ message: `Endpoint secret has been copied to clipboard`, style: 'info' });
		document.body.removeChild(el);
	}

	async deleteSubscripton() {
		this.isDeletingSubscription = true;

		try {
			const response = await this.subscriptionsService.deleteSubscription(this.activeSubscription?.uid || '');
			this.generalService.showNotification({ message: response?.message, style: 'success' });
			this.getSubscriptions();
			this.isDeletingSubscription = false;
		} catch (error) {
			this.isDeletingSubscription = false;
		}
	}
}
