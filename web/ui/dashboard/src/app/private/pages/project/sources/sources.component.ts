import { Component, OnInit, ViewChild } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { DropdownComponent } from 'src/app/components/dropdown/dropdown.component';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { SOURCE } from 'src/app/models/source.model';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { SourcesService } from './sources.service';

@Component({
	selector: 'app-sources',
	templateUrl: './sources.component.html',
	styleUrls: ['./sources.component.scss']
})
export class SourcesComponent implements OnInit {
	@ViewChild('incomingSourceDropdown') incomingSourceDropdown!: DropdownComponent;
	sourcesTableHead: string[] = ['Name', 'Type', 'Verifier', 'URL', 'Date created', ''];
	shouldShowCreateSourceModal = false;
	shouldShowUpdateSourceModal = false;
	activeSource?: SOURCE;
	sources: { content: SOURCE[]; pagination?: PAGINATION } = { content: [], pagination: undefined };
	isLoadingSources = false;
	isDeletingSource = false;
	showDeleteSourceModal = false;
	showSourceDetails = false;

	constructor(private route: ActivatedRoute, public router: Router, private sourcesService: SourcesService, public privateService: PrivateService, private generalService: GeneralService) {}

	ngOnInit() {
		this.getSources();

		const urlParam = this.route.snapshot.params.id;
		if (urlParam && urlParam === 'new') this.shouldShowCreateSourceModal = true;
		if (urlParam && urlParam !== 'new') this.shouldShowUpdateSourceModal = true;
	}

	async getSources(requestDetails?: CURSOR) {
		this.isLoadingSources = true;

		try {
			const sourcesResponse = await this.privateService.getSources(requestDetails);
			this.sources = sourcesResponse.data;
			this.isLoadingSources = false;
		} catch (error) {
			this.isLoadingSources = false;
			return error;
		}
	}

	async deleteSource() {
		this.isDeletingSource = true;
		try {
			await this.sourcesService.deleteSource(this.activeSource?.uid);
			this.isDeletingSource = false;
			this.getSources();
			this.closeModal();
			this.showDeleteSourceModal = false;
			this.activeSource = undefined;
		} catch (error) {
			this.isDeletingSource = false;
		}
	}

	closeCreateSourceModal(source: { action: string; data?: any }) {
		if (source.action !== 'close') this.generalService.showNotification({ message: `Source ${source.action}d successfully`, style: 'success' });
		this.router.navigateByUrl('/projects/' + this.privateService.activeProjectDetails?.uid + '/sources');
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
