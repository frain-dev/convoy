import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { CreatePortalLinkComponent } from 'src/app/private/components/create-portal-link/create-portal-link.component';
import { PortalLinksService } from './portal-links.service';
import { PORTAL_LINK } from 'src/app/models/endpoint.model';
import { GeneralService } from 'src/app/services/general/general.service';
import { FormsModule } from '@angular/forms';
import { DialogDirective } from 'src/app/components/dialog/dialog.directive';
import { PermissionDirective } from 'src/app/private/components/permission/permission.directive';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { formatDistanceToNowStrict } from 'date-fns';

@Component({
    selector: 'convoy-portal-links',
    imports: [CommonModule, RouterModule, FormsModule, EmptyStateComponent, CreatePortalLinkComponent, PermissionDirective, DialogDirective, ButtonComponent],
    templateUrl: './portal-links.component.html',
    styleUrls: ['./portal-links.component.scss']
})
export class PortalLinksComponent implements OnInit {
	@ViewChild('portalLinkDialog', { static: true }) portalLinkDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('deleteDialog', { static: true }) deleteDialog!: ElementRef<HTMLDialogElement>;

	isLoadingPortalLinks = false;
	fetchError = false;
	isRevokingLink = false;
	linkSearchString = '';
	portalLinks?: { pagination: PAGINATION; content: PORTAL_LINK[] };
	activeLink?: PORTAL_LINK;
	action: 'create' | 'update' = 'create';
	currentPage = 1;

	constructor(public privateService: PrivateService, public router: Router, private portalLinksService: PortalLinksService, private route: ActivatedRoute, private generalService: GeneralService, public licenseService: LicensesService) {}

	ngOnInit() {
		if (this.licenseService.hasLicense('PortalLinks')) this.getPortalLinks();

		const urlParam = this.route.snapshot.params.id;
		if (urlParam) {
			urlParam === 'new' ? (this.action = 'create') : (this.action = 'update');
			this.portalLinkDialog.nativeElement.showModal();
		}
	}

	async getPortalLinks(requestDetails?: CURSOR) {
		this.isLoadingPortalLinks = true;
		this.fetchError = false;

		try {
			const response = await this.portalLinksService.getPortalLinks({ ...requestDetails });
			this.portalLinks = response.data;
			this.isLoadingPortalLinks = false;
		} catch {
			this.fetchError = true;
			this.isLoadingPortalLinks = false;
		}
	}

	// ------- table data -------

	get displayedLinks(): PORTAL_LINK[] {
		const links = this.portalLinks?.content || [];
		const search = this.linkSearchString?.trim().toLowerCase();
		if (!search) return links;

		return links.filter(link => link.name?.toLowerCase().includes(search) || link.owner_id?.toLowerCase().includes(search));
	}

	linkCountLabel(): string {
		const total = this.portalLinks?.pagination?.total ?? this.portalLinks?.content?.length ?? 0;
		return `${total} link${total === 1 ? '' : 's'}`;
	}

	endpointCount(link: PORTAL_LINK): number {
		return link.endpoint_count || link.endpoints_metadata?.length || 0;
	}

	authTypeLabel(link: PORTAL_LINK): string {
		return link.auth_type === 'refresh_token' ? 'Refresh token' : 'Legacy';
	}

	authTypePillClass(link: PORTAL_LINK): string {
		return link.auth_type === 'refresh_token' ? 'bg-[#F5ECC6]' : 'bg-[#E3EBF2]';
	}

	lastActivity(link: PORTAL_LINK): string {
		const date = link.updated_at || link.created_at;
		if (!date) return '-';
		try {
			return formatDistanceToNowStrict(new Date(date), { addSuffix: true });
		} catch {
			return '-';
		}
	}

	// ------- row actions -------

	openPortal(link: PORTAL_LINK) {
		if (link.url) window.open(link.url, '_blank');
	}

	copyLinkUrl(link: PORTAL_LINK) {
		if (!link.url) return;
		navigator.clipboard.writeText(link.url).then(() => {
			this.generalService.showNotification({ message: 'URL has been copied to clipboard', style: 'info' });
		});
	}

	async revokeLink() {
		if (!this.activeLink) return;

		this.isRevokingLink = true;
		try {
			const response = await this.portalLinksService.revokePortalLink({ linkId: this.activeLink?.uid });
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.isRevokingLink = false;
			this.deleteDialog.nativeElement.close();
			this.getPortalLinks();
		} catch {
			this.isRevokingLink = false;
		}
	}

	// ------- pagination -------

	paginateLinks(direction: 'next' | 'prev') {
		const pagination = this.portalLinks?.pagination;
		if (!pagination) return;

		const cursor =
			direction === 'next' ? { next_page_cursor: pagination.next_page_cursor, prev_page_cursor: '', direction: 'next' as const } : { prev_page_cursor: pagination.prev_page_cursor, next_page_cursor: '', direction: 'prev' as const };

		this.currentPage = Math.max(1, this.currentPage + (direction === 'next' ? 1 : -1));
		this.getPortalLinks(cursor);
	}

	get pageRangeLabel(): string {
		const contentLength = this.portalLinks?.content?.length || 0;
		if (!contentLength) return '0 links';

		const perPage = this.portalLinks?.pagination?.per_page || contentLength;
		const start = (this.currentPage - 1) * perPage + 1;
		const end = start + contentLength - 1;
		const total = this.portalLinks?.pagination?.total;

		return total ? `${start}-${end} of ${total}` : `${start}-${end}`;
	}

	// ------- modals -------

	openCreateLinkModal() {
		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid + '/portal-links/new');
	}

	closePortalLinkModal() {
		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid + '/portal-links');
	}
}
