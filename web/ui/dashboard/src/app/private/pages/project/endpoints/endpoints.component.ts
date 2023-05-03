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
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
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
		PermissionDirective
	],
	templateUrl: './endpoints.component.html',
	styleUrls: ['./endpoints.component.scss']
})
export class EndpointsComponent implements OnInit {
	showCreateEndpointModal = this.router.url.split('/')[4] === 'new';
	showEditEndpointModal = this.router.url.split('/')[5] === 'edit';
	endpointsTableHead = ['ID', 'Status', 'Name', 'Time Created', 'Updated', ''];
	displayedEndpoints?: { date: string; content: ENDPOINT[] }[];
	endpoints?: { pagination?: PAGINATION; content?: ENDPOINT[] };
	isLoadingEndpoints = false;
	endpointSearchString!: string;

	constructor(public router: Router, public privateService: PrivateService, public projectService: ProjectService, private generalService: GeneralService, public route: ActivatedRoute) {}

	ngOnInit() {
		this.getEndpoints();
	}

	async getEndpoints(requestDetails?: CURSOR & { search?: string }) {
		this.isLoadingEndpoints = true;

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

	cancel() {
		this.router.navigateByUrl('/projects/' + this.projectService.activeProjectDetails?.uid + '/endpoints');
	}
}
