import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { FILTER_QUERY_PARAM } from 'src/app/models/event.model';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { PROJECT } from 'src/app/models/project.model';
import { SUBSCRIPTION } from 'src/app/models/subscription';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';

@Component({
	selector: 'app-subscriptions',
	templateUrl: './subscriptions.component.html',
	styleUrls: ['./subscriptions.component.scss']
})
export class SubscriptionsComponent implements OnInit {
	@ViewChild('subscriptionDialog', { static: true }) subscriptionDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('detailsDialog', { static: true }) detailsDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('deleteDialog', { static: true }) deleteDialog!: ElementRef<HTMLDialogElement>;

	activeSubscription?: SUBSCRIPTION;
	shouldShowCreateSubscriptionModal = false;
	subscriptions?: { content: SUBSCRIPTION[]; pagination?: PAGINATION };
	displayedSubscriptions?: { date: string; content: SUBSCRIPTION[] }[];
	subscriptionsLoaders = [1, 2, 3, 4, 5];
	isLoadindingSubscriptions = false;
	isDeletingSubscription = false;
	showUpdateSubscriptionModal = false;
	showDeleteSubscriptionModal = false;
	selectedSubscription?: SUBSCRIPTION;
	endpointsTableHead = ['Name', 'Status', '', '', '', '', '', ''];
	showSubscriptionDetails = false;
	projectDetails?: PROJECT;
	action: 'create' | 'update' = 'create';
	subscriptionSearchString!: string;
	userSearch = false;
	linkEndpoint?: string = this.route.snapshot.queryParams.endpointId;

	queryParams: FILTER_QUERY_PARAM = {};

	constructor(private route: ActivatedRoute, public privateService: PrivateService, public router: Router, private generalService: GeneralService, public licenseService: LicensesService) {}

	async ngOnInit() {
		this.queryParams = { ...this.route.snapshot.queryParams };

		const { name, endpointId } = this.queryParams;

		const requestDetails = { name, endpointId };

		const urlParam = this.route.snapshot.params.id;
		if (urlParam) {
			urlParam === 'new' ? (this.action = 'create') : (this.action = 'update');
			this.subscriptionDialog.nativeElement.showModal();
		}

		await this.getSubscriptions(requestDetails);

		this.route.queryParams.subscribe(params => {
			this.activeSubscription = this.subscriptions?.content.find(subscription => subscription.uid === params?.id);
			if (params.id) this.detailsDialog.nativeElement.showModal();
		});
	}

	async getSubscriptions(requestDetails?: CURSOR & { name?: string; endpointId?: string }) {
		this.isLoadindingSubscriptions = true;
		this.userSearch = !!requestDetails?.name || !!this.queryParams.name;
		this.subscriptionSearchString = this.queryParams?.name || requestDetails?.name || '';

		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, ...requestDetails });

		try {
			const subscriptionsResponse = await this.privateService.getSubscriptions(this.queryParams);
			this.subscriptions = subscriptionsResponse.data;
			this.displayedSubscriptions = this.generalService.setContentDisplayed(subscriptionsResponse.data.content, 'desc');
			this.subscriptions?.content?.length === 0 ? localStorage.setItem('isActiveProjectConfigurationComplete', 'false') : localStorage.setItem('isActiveProjectConfigurationComplete', 'true');
			this.isLoadindingSubscriptions = false;
		} catch (error) {
			this.isLoadindingSubscriptions = false;
		}
	}

	closeModal() {
		this.detailsDialog.nativeElement.close();
		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid + '/subscriptions');
	}

	createSubscription(action: any) {
		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid + '/subscriptions');
		if (action !== 'cancel') this.generalService.showNotification({ message: `Subscription has been ${action}d successfully`, style: 'success' });
	}

	async deleteSubscripton() {
		this.isDeletingSubscription = true;

		try {
			const response = await this.privateService.deleteSubscription(this.activeSubscription?.uid || '');
			this.generalService.showNotification({ message: response?.message, style: 'success' });
			this.getSubscriptions();
			delete this.activeSubscription;
			this.deleteDialog.nativeElement.close();
			this.isDeletingSubscription = false;
		} catch (error) {
			this.isDeletingSubscription = false;
		}
	}

	updateEndpointFilter(endpoint: ENDPOINT) {
		this.linkEndpoint = endpoint?.uid;
		this.getSubscriptions({ endpointId: endpoint?.uid });
	}

	clearSearch() {
		this.userSearch = false;
		this.subscriptionSearchString = '';
		delete this.queryParams['name'];
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, name: '' });

		this.getSubscriptions();
	}

	clearEndpointFilter(event?: { stopPropagation: () => void }) {
		event?.stopPropagation();
		this.linkEndpoint = undefined;
		delete this.queryParams['endpointId'];
		this.queryParams = this.generalService.addFilterToURL({ ...this.queryParams, endpointId: '' });

		this.getSubscriptions();
	}

	getEndpointSecret(endpointSecrets: any) {
		return endpointSecrets?.length === 1 ? endpointSecrets[0].value : endpointSecrets[endpointSecrets?.length - 1].value;
	}

	hasFilter(filterObject: { headers: Object; body: Object }): boolean {
		return Object.keys(filterObject.body).length > 0 || Object.keys(filterObject.headers).length > 0;
	}
}
