import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';

@Component({
	selector: 'app-sources',
	templateUrl: './sources.component.html',
	styleUrls: ['./sources.component.scss']
})
export class SourcesComponent implements OnInit {
	eventsTableHead: string[] = ['Source name', 'Source type', 'URL', 'Date created', ''];
	shouldShowCreateSourceModal = this.router.url.split('/')[4] === 'new';
	sourceID!: string;

	constructor(private route: ActivatedRoute, private router: Router) {
		this.route.queryParams.subscribe(params => {
			this.sourceID = params.id;
		});
	}

	ngOnInit(): void {}
}
