import { Component, ElementRef, EventEmitter, Input, OnInit, Output, ViewChild } from '@angular/core';
import { CommonModule, Location } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { SUBSCRIPTION } from 'src/app/models/subscription';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { PORTAL_LINK } from 'src/app/models/endpoint.model';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { PaginationComponent } from 'src/app/private/components/pagination/pagination.component';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { CreateSubscriptionModule } from 'src/app/private/components/create-subscription/create-subscription.module';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute } from '@angular/router';

@Component({
	selector: 'convoy-subscriptions-list',
	standalone: true,
	imports: [CommonModule, TagComponent, CopyButtonComponent, CardComponent, DropdownComponent, DropdownOptionDirective, ButtonComponent, PaginationComponent, DeleteModalComponent, DialogDirective, TooltipComponent, CreateSubscriptionModule, FormsModule],
	templateUrl: './subscriptions.component.html',
	styleUrls: ['./subscriptions.component.scss']
})
export class SubscriptionsComponent implements OnInit {
	@ViewChild('deleteDialog', { static: true }) deleteDialog!: ElementRef<HTMLDialogElement>;

	@Input('endpointId') endpointId = this.route.snapshot.queryParams.endpointId;
	@Input('portalDetails') portalDetails!: PORTAL_LINK;
	@Output('closeModal') closeModal = new EventEmitter();

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

	constructor(private privateService: PrivateService, private generalService: GeneralService, private location: Location, private route: ActivatedRoute) {}

	ngOnInit() {
		if (!this.endpointId) this.endpointId = this.route.snapshot.queryParams.endpointId;
		this.getSubscriptions();
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
		this.location.go(`/portal/subscriptions/${action === 'create' ? 'new' : this.activeSubscription?.uid}?token=${this.token}${action === 'create' ? '' : `&endpointId=${this.endpointId}`}`);
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
		isForm ? (this.showSubscriptionForm = false) : this.closeModal.emit();
		this.location.back();
	}
}
