import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { TableCellComponent } from 'src/app/components/table-cell/table-cell.component';
import { TableComponent } from 'src/app/components/table/table.component';
import { TableRowComponent } from 'src/app/components/table-row/table-row.component';
import { TableHeadCellComponent } from 'src/app/components/table-head-cell/table-head-cell.component';
import { PAGINATION } from 'src/app/models/global.model';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { TableHeadComponent } from 'src/app/components/table-head/table-head.component';
import { Router } from '@angular/router';
import { CreatePortalLinkComponent } from 'src/app/private/components/create-portal-link/create-portal-link.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';

@Component({
	selector: 'convoy-portal-links',
	standalone: true,
	imports: [CommonModule, ButtonComponent, CardComponent, TableLoaderModule, TableCellComponent, TableComponent, TableHeadComponent, TableRowComponent, TableHeadCellComponent, EmptyStateComponent, CreatePortalLinkComponent, ListItemComponent],
	templateUrl: './portal-links.component.html',
	styleUrls: ['./portal-links.component.scss']
})
export class PortalLinksComponent implements OnInit {
	showCreatePortalLinkModal = this.router.url.split('/')[4] === 'new';
	isLoadingPortalLinks = false;
    showPortalLinkDetails = true;
	linksTableHead = ['Link Name', 'Endpoint Count', 'URL', 'Expiration Date', ''];
	portalLinks!: { pagination: PAGINATION; content: any };

	constructor(public privateService: PrivateService, private router: Router) {}

	ngOnInit(): void {}

	getPortalLinks(requestDetails: { page: number }) {}
}
