import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { CommonModule, Location } from '@angular/common';
import { PORTAL_LINK } from 'src/app/models/endpoint.model';
import { SUBSCRIPTION } from 'src/app/models/subscription';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { ActivatedRoute } from '@angular/router';
import { CreateSubscriptionModule } from 'src/app/private/components/create-subscription/create-subscription.module';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { PaginationComponent } from 'src/app/private/components/pagination/pagination.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { FormsModule } from '@angular/forms';
import { CardComponent } from 'src/app/components/card/card.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { PortalService } from '../portal.service';
import { DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { TagComponent } from 'src/app/components/tag/tag.component';

@Component({
	selector: 'convoy-subscriptions',
	standalone: true,
	imports: [CommonModule, CreateSubscriptionModule, DeleteModalComponent, PaginationComponent, CopyButtonComponent, FormsModule, CardComponent, ButtonComponent, DropdownComponent, DropdownOptionDirective, DialogDirective, TagComponent],
	templateUrl: './subscriptions.component.html',
	styleUrls: ['./subscriptions.component.scss']
})
export class SubscriptionsComponent implements OnInit {
	@ViewChild('deleteDialog', { static: true }) deleteDialog!: ElementRef<HTMLDialogElement>;

	endpointId = this.route.snapshot.queryParams.endpointId;
	portalDetails!: PORTAL_LINK;

	isLoadingSubscriptions = false;
	isDeletingSubscription = false;
	showSubscriptionForm = false;
	subscriptionSearchString!: string;
	action: 'update' | 'create' = 'create';
	currentRoute = window.location.pathname.split('/').reverse()[0];
	activeSubscription?: SUBSCRIPTION;
	subscriptions?: { content: SUBSCRIPTION[]; pagination?: PAGINATION };
	displayedSubscriptions?: { date: string; content: SUBSCRIPTION[] }[];

	token: string = this.route.snapshot.queryParams.token;

	constructor(private privateService: PrivateService, private generalService: GeneralService, private location: Location, private route: ActivatedRoute, private portalService: PortalService) {}

	ngOnInit() {
		Promise.all([this.getPortalDetails(), this.getSubscriptions()]);
	}

	async getPortalDetails() {
		try {
			const portalLinkDetails = await this.portalService.getPortalDetail();
			this.portalDetails = portalLinkDetails.data;
		} catch (_error) {}
	}

	async getSubscriptions(requestDetails?: CURSOR & { name?: string }) {
		const endpointId = this.endpointId;
		this.isLoadingSubscriptions = true;

		try {
			const subscriptions = await this.privateService.getSubscriptions({ endpointId, ...requestDetails });

			this.subscriptions = subscriptions.data;
			this.displayedSubscriptions = this.generalService.setContentDisplayed(subscriptions.data.content);

			this.isLoadingSubscriptions = false;
		} catch {}
	}

	openSubsriptionForm(action: 'create' | 'update') {
		this.action = action;
		this.showSubscriptionForm = true;
		this.location.go(`/portal/subscriptions/${action === 'create' ? 'new' : this.activeSubscription?.uid}?token=${this.token}${this.activeSubscription || this.endpointId ? `&endpointId=${this.activeSubscription?.uid || this.endpointId}` : ''}`);
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
		} catch {
			this.isDeletingSubscription = false;
		}
	}

	goBack(isForm?: boolean) {
		if (isForm) this.showSubscriptionForm = false;
		this.location.back();
	}
}
