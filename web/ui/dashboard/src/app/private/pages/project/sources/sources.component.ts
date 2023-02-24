import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { PAGINATION } from 'src/app/models/global.model';
import { SOURCE } from 'src/app/models/group.model';
import { PrivateService } from 'src/app/private/private.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { SourcesService } from './sources.service';

@Component({
	selector: 'app-sources',
	templateUrl: './sources.component.html',
	styleUrls: ['./sources.component.scss']
})
export class SourcesComponent implements OnInit {
	sourcesTableHead: string[] = ['Name', 'Type', 'Verifier', 'URL', 'Date created', ''];
	shouldShowCreateSourceModal = false;
	shouldShowUpdateSourceModal = false;
	activeSource?: SOURCE;
	sources: { content: SOURCE[]; pagination?: PAGINATION } = { content: [], pagination: undefined };
	isLoadingSources = false;
	isDeletingSource = false;
	showDeleteSourceModal = false;
	showSourceDetails = false;

	constructor(private route: ActivatedRoute, public router: Router, private sourcesService: SourcesService, public privateService: PrivateService, private generalService: GeneralService) {
		this.route.queryParams.subscribe(params => {
			this.activeSource = this.sources?.content.find(source => source.uid === params?.id);
			params?.id && this.activeSource ? (this.showSourceDetails = true) : (this.showSourceDetails = false);
		});

		const urlParam = route.snapshot.params.id;
		if (urlParam && urlParam === 'new') this.shouldShowCreateSourceModal = true;
		if (urlParam && urlParam !== 'new') this.shouldShowUpdateSourceModal = true;

		this.getSources();
	}

	ngOnInit() {}

	async getSources(requestDetails?: { page?: number }) {
		const page = requestDetails?.page || this.route.snapshot.queryParams.page || 1;
		this.isLoadingSources = true;

		try {
			const sourcesResponse = await this.privateService.getSources({ page });
			this.sources = sourcesResponse.data;
			if ((this.sources?.pagination?.total || 0) > 0) {
				this.activeSource = this.sources?.content.find(source => source.uid === this.route.snapshot.queryParams?.id);
				if (this.route.snapshot.queryParams?.id && this.activeSource) this.showSourceDetails = true;
			}
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
}
