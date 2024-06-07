import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { ENDPOINT } from 'src/app/models/endpoint.model';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { CardComponent } from 'src/app/components/card/card.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { DialogDirective, DialogHeaderComponent } from 'src/app/components/dialog/dialog.directive';
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
import { LoaderModule } from 'src/app/private/components/loader/loader.module';

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
		CardComponent,
		EmptyStateComponent,
		DropdownComponent,
		DropdownOptionDirective,
		ListItemComponent,
		DialogHeaderComponent,
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
		DeleteModalComponent,
		LoaderModule,
		DialogDirective
	],
	templateUrl: './endpoints.component.html',
	styleUrls: ['./endpoints.component.scss']
})
export class EndpointsComponent implements OnInit {
	@ViewChild('endpointDialog', { static: true }) endpointDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('secretDialog', { static: true }) secretDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('deleteDialog', { static: true }) deleteDialog!: ElementRef<HTMLDialogElement>;

	showCreateEndpointModal = this.router.url.split('/')[4] === 'new';
	showEditEndpointModal = this.router.url.split('/')[5] === 'edit';
	endpointsTableHead = ['Name', 'Status', 'Url', 'ID', '', '', ''];
	displayedEndpoints?: { date: string; content: ENDPOINT[] }[];
	endpoints?: { pagination?: PAGINATION; content?: ENDPOINT[] };
	selectedEndpoint?: ENDPOINT;
	isLoadingEndpoints = true;
	isDeletingEndpoint = false;
	showDeleteModal = false;
	isTogglingEndpoint = false;
	isSendingTestEvent = false;
	endpointSearchString!: string;
	action: 'create' | 'update' = 'create';
	userSearch = false;

	constructor(public router: Router, public privateService: PrivateService, public projectService: ProjectService, private endpointService: EndpointsService, private generalService: GeneralService, public route: ActivatedRoute) {}

	ngOnInit() {
		const urlParam = this.route.snapshot.params.id;
		if (urlParam) {
			urlParam === 'new' ? (this.action = 'create') : (this.action = 'update');
			this.endpointDialog.nativeElement.showModal();
		}

		this.getEndpoints();
	}

	async getEndpoints(requestDetails?: CURSOR & { search?: string; hideLoader?: boolean }) {
		this.isLoadingEndpoints = !requestDetails?.hideLoader;
		this.userSearch = !!requestDetails?.search;

		try {
			const response = await this.privateService.getEndpoints({ ...requestDetails, q: requestDetails?.search || this.endpointSearchString });
			this.endpoints = response.data;
			if (response.data.content) this.displayedEndpoints = this.generalService.setContentDisplayed(response.data.content, 'desc');
			this.isLoadingEndpoints = false;
		} catch {
			this.isLoadingEndpoints = false;
		}
	}

	searchEndpoint(searchDetails: { searchInput?: any }) {
		const searchString: string = searchDetails?.searchInput?.target?.value || this.endpointSearchString;
		this.getEndpoints({ search: searchString, hideLoader: true });
	}

	async deleteEndpoint() {
		if (!this.selectedEndpoint) return;
		this.isDeletingEndpoint = true;

		try {
			const response = await this.endpointService.deleteEndpoint(this.selectedEndpoint?.uid || '');
			this.getEndpoints({ hideLoader: true });

			this.generalService.showNotification({ style: 'success', message: response.message });
			this.deleteDialog.nativeElement.close();
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
		this.endpointDialog.nativeElement.close();
		this.router.navigateByUrl('/projects/' + this.projectService.activeProjectDetails?.uid + '/endpoints');
	}
}
