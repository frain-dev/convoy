import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { PAGINATION } from 'src/app/models/global.model';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { TableComponent, TableCellComponent, TableRowComponent, TableHeadCellComponent, TableHeadComponent } from 'src/app/components/table/table.component';
import { ActivatedRoute, Router } from '@angular/router';
import { CreatePortalLinkComponent } from 'src/app/private/components/create-portal-link/create-portal-link.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { PortalLinksService } from './portal-links.service';

@Component({
	selector: 'convoy-portal-links',
	standalone: true,
	imports: [CommonModule, ButtonComponent, CardComponent, TableLoaderModule, TableComponent, TableHeadComponent, TableRowComponent, TableHeadCellComponent, TableCellComponent, EmptyStateComponent, CreatePortalLinkComponent, ListItemComponent],
	templateUrl: './portal-links.component.html',
	styleUrls: ['./portal-links.component.scss']
})
export class PortalLinksComponent implements OnInit {
	showCreatePortalLinkModal = this.router.url.split('/')[4] === 'new';
	isLoadingPortalLinks = false;
	showPortalLinkDetails = true;
	linksTableHead = ['Link Name', 'Endpoint Count', 'URL', 'Expiration Date', ''];
	portalLinks!: { pagination: PAGINATION; content: any };

	constructor(public privateService: PrivateService, private router: Router, private portalLinksService: PortalLinksService, private route: ActivatedRoute) {}

	ngOnInit() {
		this.getPortalLinks();
	}

	async getPortalLinks(requestDetails?: { search?: string; page?: number }) {
		this.isLoadingPortalLinks = true;
		const page = requestDetails?.page || this.route.snapshot.queryParams.page || 1;
		try {
			const response = await this.portalLinksService.getPortalLinks({ pageNo: page, searchString: requestDetails?.search });
			this.isLoadingPortalLinks = false;
			console.log(response);
		} catch {
			this.isLoadingPortalLinks = false;
		}
	}
}
