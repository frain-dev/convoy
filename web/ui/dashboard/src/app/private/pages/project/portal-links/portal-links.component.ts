import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { CardComponent } from 'src/app/components/card/card.component';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { CreatePortalLinkComponent } from 'src/app/private/components/create-portal-link/create-portal-link.component';
import { ListItemComponent } from 'src/app/components/list-item/list-item.component';
import { PortalLinksService } from './portal-links.service';
import { ENDPOINT, PORTAL_LINK } from 'src/app/models/endpoint.model';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { DeleteModalComponent } from 'src/app/private/components/delete-modal/delete-modal.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { FormsModule } from '@angular/forms';
import { DropdownComponent, DropdownOptionDirective } from 'src/app/components/dropdown/dropdown.component';
import { fromEvent, Observable } from 'rxjs';
import { debounceTime, distinctUntilChanged, map, startWith, switchMap } from 'rxjs/operators';
import { ModalComponent, ModalHeaderComponent } from 'src/app/components/modal/modal.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { PaginationComponent } from 'src/app/private/components/pagination/pagination.component';

@Component({
	selector: 'convoy-portal-links',
	standalone: true,
	imports: [
		CommonModule,
		RouterModule,
		FormsModule,
		ButtonComponent,
		DropdownComponent,
		DropdownOptionDirective,
		CardComponent,
		EmptyStateComponent,
		CreatePortalLinkComponent,
		ListItemComponent,
		CopyButtonComponent,
		DeleteModalComponent,
		ModalComponent,
		ModalHeaderComponent,
		TooltipComponent,
		PaginationComponent
	],
	templateUrl: './portal-links.component.html',
	styleUrls: ['./portal-links.component.scss']
})
export class PortalLinksComponent implements OnInit {
	showCreatePortalLinkModal = this.router.url.split('/')[4] === 'new';
	showEditPortalLinkModal = this.router.url.split('/')[5] === 'edit';
	isLoadingPortalLinks = false;
	showDeleteModal = false;
	isRevokingLink = false;
	linkEndpoint?: string = this.route.snapshot.queryParams.linksEndpoint;
	linkSearchString!: string;
	linksTableHead = ['Link Name', 'Endpoints', 'URL', 'Created', ''];
	portalLinks?: { pagination: PAGINATION; content: PORTAL_LINK[] };
	activeLink?: PORTAL_LINK;
	@ViewChild('linksEndpointFilter', { static: true }) linksEndpointFilter!: ElementRef;
	linksEndpointFilter$!: Observable<ENDPOINT[]>;

	constructor(public privateService: PrivateService, public router: Router, private portalLinksService: PortalLinksService, private route: ActivatedRoute, private generalService: GeneralService) {
		this.route.queryParams.subscribe(params => (this.activeLink = this.portalLinks?.content.find(link => link.uid === params?.id)));
	}

	ngOnInit() {
		this.getPortalLinks();
	}

	ngAfterViewInit() {
		this.linksEndpointFilter$ = fromEvent<any>(this.linksEndpointFilter?.nativeElement, 'keyup').pipe(
			map(event => event.target.value),
			startWith(''),
			debounceTime(500),
			distinctUntilChanged(),
			switchMap(search => this.getEndpointsForFilter(search))
		);
	}

	async getPortalLinks(requestDetails?: CURSOR) {
		this.isLoadingPortalLinks = true;

		try {
			const response = await this.portalLinksService.getPortalLinks({ ...requestDetails, endpointId: this.linkEndpoint });
			this.portalLinks = response.data;
			if ((this.portalLinks?.pagination?.total || 0) > 0) this.activeLink = this.portalLinks?.content.find(link => link.uid === this.route.snapshot.queryParams?.id);
			this.isLoadingPortalLinks = false;
		} catch {
			this.isLoadingPortalLinks = false;
		}
	}

	async revokeLink() {
		if (!this.activeLink) return;

		this.isRevokingLink = true;
		try {
			const response = await this.portalLinksService.revokePortalLink({ linkId: this.activeLink?.uid });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.isRevokingLink = false;
			this.showDeleteModal = false;
			this.getPortalLinks();
		} catch {
			this.isRevokingLink = false;
		}
	}

	searchLinks(searchDetails: { searchInput?: any }) {
		const searchString: string = searchDetails?.searchInput?.target?.value || this.linkSearchString;
		// not in use yet
		// this.getPortalLinks({ search: searchString });
	}

	async getEndpointsForFilter(search: string): Promise<ENDPOINT[]> {
		return await (
			await this.privateService.getEndpoints({ q: search })
		).data.content;
	}

	updateEndpointFilter(endpointId: string) {
		this.linkEndpoint = endpointId;
		this.getPortalLinks();
	}

	clearEndpointFilter() {
		this.linkEndpoint = undefined;
		this.getPortalLinks();
		this.router.navigate([], { relativeTo: this.route, queryParams: {} });
	}

	openCreateLinkModal() {
		this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails?.uid + '/portal-links/new');
	}

	viewEndpoint(endpoint: ENDPOINT) {
		this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails?.uid + '/endpoints/' + endpoint.uid);
	}
}
