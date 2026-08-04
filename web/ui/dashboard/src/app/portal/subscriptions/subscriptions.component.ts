import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { Location } from '@angular/common';
import { PORTAL_LINK } from 'src/app/models/endpoint.model';
import { SUBSCRIPTION } from 'src/app/models/subscription';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { ActivatedRoute } from '@angular/router';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { FormsModule } from '@angular/forms';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { PortalService } from '../portal.service';
import { DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { LicensesService } from '../../services/licenses/licenses.service';
import {CreatePortalEndpointComponent} from "../create-portal-endpoint/create-portal-endpoint.component";

@Component({
    selector: 'convoy-subscriptions',
    imports: [DeleteModalComponent, CopyButtonComponent, FormsModule, DropdownComponent, DropdownOptionDirective, DialogDirective, CreatePortalEndpointComponent],
    templateUrl: './subscriptions.component.html',
    styleUrls: ['./subscriptions.component.scss']
})
export class SubscriptionsComponent implements OnInit {
	@ViewChild('deleteDialog', { static: true }) deleteDialog!: ElementRef<HTMLDialogElement>;

	endpointId = this.route.snapshot.queryParams.endpointId;
	portalDetails?: PORTAL_LINK;

	isLoadingSubscriptions = false;
	subscriptionsLoadFailed = false;
	isDeletingSubscription = false;
	showSubscriptionForm = false;
	subscriptionSearchString!: string;
	action: 'update' | 'create' = 'create';
	currentRoute = window.location.pathname.split('/').reverse()[0];
	activeSubscription?: SUBSCRIPTION;
	subscriptions?: { content: SUBSCRIPTION[]; pagination?: PAGINATION };
	displayedSubscriptions?: { date: string; content: SUBSCRIPTION[] }[];
	// Bumps on every getSubscriptions call so a late error/success from an
	// older search/refetch cannot wipe or restore stale rows.
	private subscriptionsRequestToken = 0;

	token: string = this.route.snapshot.queryParams.token;

	constructor(private privateService: PrivateService, private generalService: GeneralService, private location: Location, private route: ActivatedRoute, private portalService: PortalService, public licenseService: LicensesService) {}

	ngOnInit() {
		Promise.all([this.getPortalDetails(), this.getSubscriptions(), this.licenseService.setLicenses()]);
	}

	async getPortalDetails() {
		try {
			const portalLinkDetails = await this.portalService.getPortalDetail();
			this.portalDetails = portalLinkDetails.data;
		} catch (_error) {}
	}

	async getSubscriptions(requestDetails?: CURSOR & { name?: string }) {
		const endpointId = this.endpointId;
		const requestToken = ++this.subscriptionsRequestToken;
		this.isLoadingSubscriptions = true;
		this.subscriptionsLoadFailed = false;

		try {
			const subscriptions = await this.privateService.getSubscriptions({ endpointId, ...requestDetails });
			if (requestToken !== this.subscriptionsRequestToken) return;

			this.subscriptions = subscriptions.data;
			this.displayedSubscriptions = this.generalService.setContentDisplayed(subscriptions.data.content, 'desc');
			this.subscriptionsLoadFailed = false;
		} catch {
			if (requestToken !== this.subscriptionsRequestToken) return;
			// Fail closed on display: drop spinner and prior rows so a failed search
			// or refetch cannot leave stale results under the current query. Distinct
			// from empty success so the template does not claim "No subscriptions yet".
			this.subscriptions = undefined;
			this.displayedSubscriptions = [];
			this.subscriptionsLoadFailed = true;
		} finally {
			if (requestToken === this.subscriptionsRequestToken) {
				this.isLoadingSubscriptions = false;
			}
		}
	}

	openSubsriptionForm(action: 'create' | 'update') {
        let project = localStorage.getItem("CONVOY_PROJECT");
        if (!project && this.activeSubscription?.project_id) {
            localStorage.setItem(
                'CONVOY_PROJECT',
                JSON.stringify({ uid: this.activeSubscription?.project_id })
            );
        }
		this.action = action;
		this.showSubscriptionForm = true;
        let subscriptionPath = '/portal/subscriptions/';
        if (action === 'create') {
            subscriptionPath += 'new';
        } else if (this.activeSubscription?.uid) {
            subscriptionPath += this.activeSubscription.uid;
        }

        let queryParams = `?token=${this.token}`;
        if (this.endpointId) {
            queryParams += `&endpointId=${this.endpointId}`;
        }

        this.location.go(subscriptionPath + queryParams);
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
