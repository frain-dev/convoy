import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { CardComponent } from 'src/app/components/card/card.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { ModalComponent, ModalHeaderComponent } from 'src/app/components/modal/modal.component';
import { CreateEndpointComponent } from 'src/app/private/components/create-endpoint/create-endpoint.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { FormsModule } from '@angular/forms';
import { TableComponent, TableCellComponent, TableRowComponent, TableHeadCellComponent, TableHeadComponent } from 'src/app/components/table/table.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { ProjectService } from '../project.service';
import { PaginationComponent } from 'src/app/private/components/pagination/pagination.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { PermissionDirective } from 'src/app/private/components/permission/permission.directive';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { EndpointSecretComponent } from './endpoint-secret/endpoint-secret.component';
import { EndpointsService } from './endpoints.service';

@Component({
	selector: 'convoy-endpoints',
	standalone: true,
	imports: [
		CommonModule,
		ButtonComponent,
		TableCellComponent,
		TableHeadComponent,
		TableHeadCellComponent,
		TableRowComponent,
		TableCellComponent,
		TableComponent,
		TableLoaderModule,
		CardComponent,
		EmptyStateComponent,
		DropdownComponent,
		DropdownOptionDirective,
		ListItemComponent,
		ModalComponent,
		ModalHeaderComponent,
		CreateEndpointComponent,
		TagComponent,
		FormsModule,
		RouterModule,
		StatusColorModule,
		TooltipComponent,
		PaginationComponent,
		CopyButtonComponent,
		PermissionDirective,
		EndpointSecretComponent,
		DeleteModalComponent
	],
	templateUrl: './endpoints.component.html',
	styleUrls: ['./endpoints.component.scss']
})
export class EndpointsComponent implements OnInit {
	showCreateEndpointModal = this.router.url.split('/')[4] === 'new';
	showEditEndpointModal = this.router.url.split('/')[5] === 'edit';
	endpointsTableHead = ['Name', 'Status', 'ID', '', '', ''];
	displayedEndpoints?: { date: string; content: ENDPOINT[] }[];
	endpoints?: { pagination?: PAGINATION; content?: ENDPOINT[] };
	selectedEndpoint?: ENDPOINT;
	isLoadingEndpoints = false;
	showEndpointSecret = false;
	isDeletingEndpoint = false;
	showDeleteModal = false;
	isTogglingEndpoint = false;
	isSendingTestEvent = false;
	endpointSearchString!: string;

	constructor(public router: Router, public privateService: PrivateService, public projectService: ProjectService, private endpointService: EndpointsService, private generalService: GeneralService, public route: ActivatedRoute) {}

	ngOnInit() {
		this.getEndpoints();
	}

	async getEndpoints(requestDetails?: CURSOR & { search?: string; showLoader?: boolean }) {
		if (requestDetails?.showLoader) this.isLoadingEndpoints = true;

		try {
			const response = await this.privateService.getEndpoints({ ...requestDetails, q: requestDetails?.search || this.endpointSearchString });
			this.endpoints = response.data;
			this.displayedEndpoints = this.generalService.setContentDisplayed(response.data.content);
			this.isLoadingEndpoints = false;
		} catch {
			this.isLoadingEndpoints = false;
		}
	}

	searchEndpoint(searchDetails: { searchInput?: any }) {
		const searchString: string = searchDetails?.searchInput?.target?.value || this.endpointSearchString;
		this.getEndpoints({ search: searchString });
	}

	async deleteEndpoint() {
		if (!this.selectedEndpoint) return;
		this.isDeletingEndpoint = true;

		try {
			const response = await this.endpointService.deleteEndpoint(this.selectedEndpoint?.uid || '');
			this.getEndpoints({ showLoader: false });

			this.generalService.showNotification({ style: 'success', message: response.message });
			this.showDeleteModal = false;
			this.isDeletingEndpoint = false;
		} catch {
			this.isDeletingEndpoint = false;
		}
	}

	async toggleEndpoint() {
		this.isTogglingEndpoint = true;
		if (!this.selectedEndpoint?.uid) return;

		try {
			const response = await this.endpointService.toggleEndpoint(this.selectedEndpoint?.uid);
			this.displayedEndpoints?.forEach(item => {
				item.content.forEach(endpoint => {
					if (response.data.uid === endpoint.uid) endpoint.status = response.data.status;
				});
			});
			this.generalService.showNotification({ message: `${this.selectedEndpoint?.title} status updated successfully`, style: 'success' });
			this.isTogglingEndpoint = false;
		} catch {
			this.isTogglingEndpoint = false;
		}
	}

	async sendTestEvent() {
		const testEvent = {
			data: { data: 'test event from Convoy', convoy: 'https://getconvoy.io', amount: 1000 },
			endpoint_id: this.selectedEndpoint?.uid,
			event_type: 'test.convoy'
		};

		this.isSendingTestEvent = true;
		try {
			const response = await this.endpointService.sendEvent({ body: testEvent });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.isSendingTestEvent = false;
		} catch {
			this.isSendingTestEvent = false;
		}
	}

	cancel() {
		this.router.navigateByUrl('/projects/' + this.projectService.activeProjectDetails?.uid + '/endpoints');
	}
}
