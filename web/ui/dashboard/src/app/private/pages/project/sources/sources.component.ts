import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { SOURCE } from 'src/app/models/source.model';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { SourcesService } from './sources.service';
import { PROJECT } from 'src/app/models/project.model';

@Component({
	selector: 'app-sources',
	templateUrl: './sources.component.html',
	styleUrls: ['./sources.component.scss']
})
export class SourcesComponent implements OnInit {
	@ViewChild('incomingSourceDropdown') incomingSourceDropdown!: DropdownComponent;
	@ViewChild('sourceDialog', { static: true }) sourceDialog!: ElementRef<HTMLDialogElement>;
	@ViewChild('deleteDialog', { static: true }) deleteDialog!: ElementRef<HTMLDialogElement>;

	sourcesTableHead: string[] = ['Name', 'Type', 'Verifier', 'URL', 'Date created', ''];
	activeSource?: SOURCE;
	sources: { content: SOURCE[]; pagination?: PAGINATION } = { content: [], pagination: undefined };
	isLoadingSources = false;
	isDeletingSource = false;
	showDeleteSourceModal = false;
	showSourceDetails = false;
	projectDetails?: PROJECT;
	action: 'create' | 'update' = 'create';

	constructor(private route: ActivatedRoute, public router: Router, private sourcesService: SourcesService, public privateService: PrivateService, private generalService: GeneralService) {}

	ngOnInit() {
		this.getSources();

		const urlParam = this.route.snapshot.params.id;
		if (urlParam) {
			urlParam === 'new' ? (this.action = 'create') : (this.action = 'update');
			this.sourceDialog.nativeElement.showModal();
		}
	}

	async getSources(requestDetails?: CURSOR) {
		this.isLoadingSources = true;

		try {
			const sourcesResponse = await this.privateService.getSources(requestDetails);
			this.sources = sourcesResponse.data;
			this.isLoadingSources = false;
		} catch {
			this.isLoadingSources = false;
			return;
		}
	}

	async deleteSource() {
		this.isDeletingSource = true;
		try {
			await this.sourcesService.deleteSource(this.activeSource?.uid);
			this.isDeletingSource = false;
			this.getSources();
			this.closeModal();
			this.deleteDialog.nativeElement.close();
			this.activeSource = undefined;
		} catch (error) {
			this.isDeletingSource = false;
		}
	}

	closeCreateSourceModal(source: { action: string; data?: any }) {
		if (source.action !== 'close') this.generalService.showNotification({ message: `Source ${source.action}d successfully`, style: 'success' });
		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.uid + '/sources');
	}

	isDateBefore(date1?: Date, date2?: Date): boolean {
		if (date1 && date2) return date1 > date2;
		return false;
	}

	closeModal() {
		this.router.navigate([], { queryParams: {} });
	}

	paginate(event: PAGINATION) {
		this.getSources();
	}

	hideIncomingSourceDropdown() {
		this.incomingSourceDropdown.show = false;
	}
}
