import { Component, HostListener, OnInit } from '@angular/core';
import { CommonModule, Location } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { CardComponent } from 'src/app/components/card/card.component';
import { EndpointDetailsService } from './endpoint-details.service';
import { ENDPOINT } from 'src/app/models/app.model';
import { ActivatedRoute } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { CliKeysComponent } from './cli-keys/cli-keys.component';
import { DevicesComponent } from './devices/devices.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { SendEventComponent } from 'src/app/private/components/send-event/send-event.component';

@Component({
	selector: 'convoy-endpoint-details',
	standalone: true,
	imports: [CommonModule, CardComponent, ButtonComponent, SkeletonLoaderComponent, CliKeysComponent, DevicesComponent, ModalComponent, SendEventComponent],
	templateUrl: './endpoint-details.component.html',
	styleUrls: ['./endpoint-details.component.scss']
})
export class EndpointDetailsComponent implements OnInit {
	isLoadingEndpointDetails = false;
	isCliAvailable = false;
	shouldRenderSmallSize = false;
    showAddEventModal = true;
	endpointDetails!: ENDPOINT;
	screenWidth = window.innerWidth;
	tabs: ['CLI Keys', 'devices'] = ['CLI Keys', 'devices'];
	activeTab: 'CLI Keys' | 'devices' = 'CLI Keys';

	constructor(public privateService: PrivateService, private endpointDetailsService: EndpointDetailsService, private route: ActivatedRoute, private location: Location) {}

	async ngOnInit() {
		this.isLoadingEndpointDetails = true;
		this.isCliAvailable = await this.privateService.getFlag('can_create_cli_api_key');
		this.checkScreenSize();
		this.getEndpointDetails(this.route.snapshot.params.id);
	}

	async getEndpointDetails(endpointId: string) {
		this.isLoadingEndpointDetails = true;

		try {
			const response = await this.endpointDetailsService.getEndpoint(endpointId);
			this.endpointDetails = response.data;
			this.isLoadingEndpointDetails = false;
		} catch {
			this.isLoadingEndpointDetails = false;
		}
	}

	toggleActiveTab(tab: 'CLI Keys' | 'devices') {
		this.activeTab = tab;
	}

	goBack() {
		this.location.back();
	}

	checkScreenSize() {
		this.screenWidth > 1010 ? (this.shouldRenderSmallSize = false) : (this.shouldRenderSmallSize = true);
	}

	@HostListener('window:resize', ['$event'])
	onWindowResize() {
		this.screenWidth = window.innerWidth;
		this.checkScreenSize();
	}
}
