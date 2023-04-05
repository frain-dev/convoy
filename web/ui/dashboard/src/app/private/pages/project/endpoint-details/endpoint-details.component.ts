import { Component, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { CardComponent } from 'src/app/components/card/card.component';
import { EndpointDetailsService } from './endpoint-details.service';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { CliKeysComponent } from './cli-keys/cli-keys.component';
import { DevicesComponent } from './devices/devices.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { SendEventComponent } from 'src/app/private/components/send-event/send-event.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { EndpointSecretComponent } from './endpoint-secret/endpoint-secret.component';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { TagComponent } from 'src/app/components/tag/tag.component';

@Component({
	selector: 'convoy-endpoint-details',
	standalone: true,
	imports: [
		CommonModule,
		RouterModule,
		CardComponent,
		ButtonComponent,
		SkeletonLoaderComponent,
		CliKeysComponent,
		DevicesComponent,
		ModalComponent,
		SendEventComponent,
		DeleteModalComponent,
		CopyButtonComponent,
		EndpointSecretComponent,
		DropdownComponent,
		DropdownOptionDirective,
		TagComponent
	],
	templateUrl: './endpoint-details.component.html',
	styleUrls: ['./endpoint-details.component.scss']
})
export class EndpointDetailsComponent implements OnInit {
	@ViewChild(CliKeysComponent) cliKeys!: CliKeysComponent;
	isLoadingEndpointDetails = false;
	isCliAvailable = false;
	showAddEventModal = false;
	isDeletingEndpoint = false;
	showDeleteModal = false;
	showEndpointSecret = false;
	endpointDetails?: ENDPOINT;
	secretKey: any;
	endpointId = this.route.snapshot.params.id;
	screenWidth = window.innerWidth;
	tabs: ['Keys', 'devices'] = ['Keys', 'devices'];
	activeTab: 'Keys' | 'devices' = 'Keys';
	isSendingTestEvent = false;
	isTogglingEndpoint = false;

	constructor(public privateService: PrivateService, private endpointDetailsService: EndpointDetailsService, public route: ActivatedRoute, private router: Router, private generalService: GeneralService) {}

	async ngOnInit() {
		this.isLoadingEndpointDetails = true;
		this.isCliAvailable = await this.privateService.getFlag('can_create_cli_api_key');
		this.getEndpointDetails();
	}

	async getEndpointDetails() {
		this.isLoadingEndpointDetails = true;

		try {
			const response = await this.endpointDetailsService.getEndpoint(this.endpointId);
			this.endpointDetails = response.data;
			const secrets = response.data.secrets;
			this.secretKey = secrets[secrets.length - 1];
			this.isLoadingEndpointDetails = false;
		} catch {
			this.isLoadingEndpointDetails = false;
		}
	}
	async deleteEndpoint() {
		if (!this.endpointDetails) return;
		this.isDeletingEndpoint = true;

		try {
			const response = await this.endpointDetailsService.deleteEndpoint(this.endpointDetails?.uid || '');
			this.generalService.showNotification({ style: 'success', message: response.message });
			this.showDeleteModal = false;
			this.isDeletingEndpoint = false;
			this.goBack();
		} catch {
			this.isDeletingEndpoint = false;
		}
	}

	viewEndpointEvents(endpointUid?: string) {
		if (endpointUid) this.router.navigate(['../../events'], { relativeTo: this.route, queryParams: { eventDelsEndpoint: endpointUid } });
	}

	viewEndpointPortalLinks(endpointUid?: string) {
		if (endpointUid) this.router.navigate(['../../portal-links'], { relativeTo: this.route, queryParams: { linksEndpoint: endpointUid } });
	}

	toggleActiveTab(tab: 'Keys' | 'devices') {
		this.activeTab = tab;
	}

	goBack() {
		this.router.navigate(['../../endpoints'], { relativeTo: this.route });
	}

	async sendTestEvent() {
		const testEvent = {
			data: { data: 'test event from Convoy', convoy: 'https://getconvoy.io', amount: 1000 },
			endpoint_id: this.endpointDetails?.uid,
			event_type: 'test.convoy'
		};

		this.isSendingTestEvent = true;
		try {
			const response = await this.endpointDetailsService.sendEvent({ body: testEvent });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.isSendingTestEvent = false;
		} catch {
			this.isSendingTestEvent = false;
		}
	}

	async toggleEndpoint() {
		this.isTogglingEndpoint = true;
		if (!this.endpointDetails?.uid) return;

		try {
			const response = await this.endpointDetailsService.toggleEndpoint(this.endpointDetails?.uid);
			this.endpointDetails = response.data;
			this.generalService.showNotification({ message: `${this.endpointDetails?.title} puased successfully`, style: 'success' });
			this.isTogglingEndpoint = false;
		} catch {
			this.isTogglingEndpoint = false;
		}
	}
}
