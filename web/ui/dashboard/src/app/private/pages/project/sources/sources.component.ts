import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';
import { PAGINATION } from 'convoy-app/lib/models/global.model';
import { SOURCE } from 'src/app/models/group.model';
import { PrivateService } from 'src/app/private/private.service';
import { SourcesService } from './sources.service';

@Component({
	selector: 'app-sources',
	templateUrl: './sources.component.html',
	styleUrls: ['./sources.component.scss']
})
export class SourcesComponent implements OnInit {
	sourcesTableHead: string[] = ['Source name', 'Source type', 'Verifier', 'URL', 'Date created', ''];
	shouldShowCreateSourceModal = this.router.url.split('/')[4] === 'new';
	activeSource?: SOURCE;
	sources!: { content: SOURCE[]; pagination: PAGINATION };
	isLoadingSources = false;
	projectId = this.privateService.activeProjectId;

	constructor(private route: ActivatedRoute, private router: Router, private sourcesService: SourcesService, private privateService: PrivateService) {
		this.route.queryParams.subscribe(params => {
			this.activeSource = this.sources?.content.find(source => source.uid === params?.id);
		});
	}

	ngOnInit() {
		this.getSources();
	}

	async getSources(requestDetails?: { page?: number }) {
		const page = requestDetails?.page || this.route.snapshot.queryParams.page || 1;
		this.isLoadingSources = true;
		try {
			const sourcesResponse = await this.privateService.getSources({ page });
			this.sources = sourcesResponse.data;
			if (this.sources.pagination.total > 0) this.activeSource = this.sources?.content.find(source => source.uid === this.route.snapshot.queryParams?.id);
			this.isLoadingSources = false;
		} catch (error) {
			this.isLoadingSources = false;
			return error;
		}
	}

	async deleteSource() {
		try {
			await this.sourcesService.deleteSource(this.activeSource?.uid);
			this.getSources();
			this.router.navigateByUrl('./');
			this.activeSource = undefined;
		} catch (error) {
			console.log(error);
		}
	}

	closeCreateSourceModal() {
		this.router.navigateByUrl('/projects/' + this.projectId + '/sources');
	}
}
