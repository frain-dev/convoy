import { Component, HostListener, OnInit, ViewChild } from '@angular/core';
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

@Component({
	selector: 'convoy-endpoint-details',
	standalone: true,
	imports: [CommonModule, RouterModule, CardComponent, ButtonComponent, SkeletonLoaderComponent, CliKeysComponent, DevicesComponent, ModalComponent, SendEventComponent, DeleteModalComponent, CopyButtonComponent, EndpointSecretComponent],
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
	screenWidth = window.innerWidth;
	tabs: ['CLI Keys', 'devices'] = ['CLI Keys', 'devices'];
	activeTab: 'CLI Keys' | 'devices' = 'CLI Keys';

	constructor(public privateService: PrivateService, private endpointDetailsService: EndpointDetailsService, private route: ActivatedRoute, private router: Router, private generalService: GeneralService) {}

	async ngOnInit() {
		this.isLoadingEndpointDetails = true;
		this.isCliAvailable = await this.privateService.getFlag('can_create_cli_api_key');
		this.getEndpointDetails(this.route.snapshot.params.id);
	}

	async getEndpointDetails(endpointId: string) {
		this.isLoadingEndpointDetails = true;

		try {
			const response = await this.endpointDetailsService.getEndpoint(endpointId);
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
			this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails?.uid + '/endpoints');
		} catch {
			this.isDeletingEndpoint = false;
		}
	}

	viewEndpointEvents(endpointUid?: string) {
		if (endpointUid) this.router.navigate(['/projects/' + this.privateService.activeProjectDetails?.uid + '/events'], { queryParams: { eventDelsEndpoint: endpointUid } });
	}

	viewEndpointPortalLinks(endpointUid?: string) {
		if (endpointUid) this.router.navigate(['/projects/' + this.privateService.activeProjectDetails?.uid + '/portal-links'], { queryParams: { linksEndpoint: endpointUid } });
	}

	toggleActiveTab(tab: 'CLI Keys' | 'devices') {
		this.activeTab = tab;
	}

	goBack() {
		this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails?.uid + '/endpoints');
	}
}
