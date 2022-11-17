import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { Router } from '@angular/router';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { TableCellComponent } from 'src/app/components/table-cell/table-cell.component';
import { TableComponent } from 'src/app/components/table/table.component';
import { TableHeadCellComponent } from 'src/app/components/table-head-cell/table-head-cell.component';
import { TableHeadComponent } from 'src/app/components/table-head/table-head.component';
import { TableRowComponent } from 'src/app/components/table-row/table-row.component';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { ENDPOINT } from 'src/app/models/app.model';
import { PAGINATION } from 'src/app/models/global.model';
import { CardComponent } from 'src/app/components/card/card.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { CreateEndpointComponent } from 'src/app/private/components/create-endpoint/create-endpoint.component';
import { EndpointsService } from './endpoints.service';

@Component({
	selector: 'convoy-endpoints',
	standalone: true,
	imports: [
		CommonModule,
		ButtonComponent,
		TableCellComponent,
		TableComponent,
		TableHeadCellComponent,
		TableHeadComponent,
		TableRowComponent,
		TableLoaderModule,
		TableRowComponent,
		CardComponent,
		EmptyStateComponent,
		DropdownComponent,
		ListItemComponent,
		ModalComponent,
		CreateEndpointComponent
	],
	templateUrl: './endpoints.component.html',
	styleUrls: ['./endpoints.component.scss']
})
export class EndpointsComponent implements OnInit {
	showCreateEndpointModal = this.router.url.split('/')[4] === 'new';
	showEditEndpointModal = this.router.url.split('/')[5] === 'edit';
	endpointsTableHead = ['Status', 'Name', 'Time Created', 'Updated', 'Events', '', ''];
	displayedEndpoints: { date: string; content: ENDPOINT[] }[] = [];
	endpoints!: { pagination: PAGINATION; content: ENDPOINT[] };
	isLoadingEndpoints = false;

	constructor(public privateService: PrivateService, private router: Router, private endpointService: EndpointsService) {}

	ngOnInit() {
		this.getEndpoints();
	}

	async getEndpoints(requestDetails?: { page?: number }) {
		try {
			const response = await this.endpointService.getEndpoints();
			console.log(response);
		} catch {}
	}
}
