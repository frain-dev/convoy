import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { CardComponent } from 'src/app/components/card/card.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { MetaEventsService } from './meta-events.service';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { TableCellComponent, TableComponent, TableHeadCellComponent, TableHeadComponent, TableRowComponent } from 'src/app/components/table/table.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { PrismModule } from 'src/app/private/components/prism/prism.module';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { PaginationComponent } from 'src/app/private/components/pagination/pagination.component';
import { META_EVENT } from 'src/app/models/project.model';
import { CURSOR, PAGINATION } from 'src/app/models/global.model';
import { Router } from '@angular/router';
import { DialogHeaderComponent } from 'src/app/components/dialog/dialog.directive';

@Component({
	selector: 'convoy-meta-events',
	standalone: true,
	imports: [
		CommonModule,
		CardComponent,
		EmptyStateComponent,
		ButtonComponent,
		TooltipComponent,
		DialogHeaderComponent,
		TableLoaderModule,
		TableCellComponent,
		TableComponent,
		TableHeadComponent,
		TableHeadCellComponent,
		TableRowComponent,
		TagComponent,
		StatusColorModule,
		PrismModule,
		PaginationComponent
	],
	templateUrl: './meta-events.component.html',
	styleUrls: ['./meta-events.component.scss']
})
export class MetaEventsComponent implements OnInit {
	metaEventsTableHead: string[] = ['Status', 'Event Types', 'Retries', 'Time', '', ''];
	showMetaConfig = false;
	isLoadingMetaEvents = false;
	isRetryingMetaEvent = false;
	metaEvents!: { pagination: PAGINATION; content: META_EVENT[] };
	displayedMetaEvents!: { date: string; content: META_EVENT[] }[];
	selectedMetaEvent: any;

	constructor(public privateService: PrivateService, public generalService: GeneralService, private metaEventsService: MetaEventsService, private router: Router) {}

	ngOnInit(): void {
		this.getMetaEvents();
	}

	get isMetaEventEnabled(): Boolean {
		const isMetaEventEnabled = this.privateService.getProjectDetails?.config?.meta_event?.is_enabled || false;
		return isMetaEventEnabled;
	}

	toggleMetaConfig(event: any) {
		const isConfigfureMetaEventsChecked = event.target.checked;
		this.showMetaConfig = isConfigfureMetaEventsChecked;
	}

	async getMetaEvents(requestDetails?: CURSOR) {
		this.isLoadingMetaEvents = true;
		try {
			const response = await this.metaEventsService.getMetaEvents(requestDetails);
			this.metaEvents = response.data;
			if (this.metaEvents?.content?.length) this.selectedMetaEvent = this.metaEvents?.content[0];
			this.displayedMetaEvents = await this.generalService.setContentDisplayed(response.data.content);
			this.isLoadingMetaEvents = false;
		} catch {
			this.isLoadingMetaEvents = false;
		}
	}

	async retryMetaEvent(metaEventId: string) {
		this.isRetryingMetaEvent = true;
		try {
			const response = await this.metaEventsService.retryMetaEvent(metaEventId);
			this.isRetryingMetaEvent = false;
			this.generalService.showNotification({ message: response.message, style: 'success' });
			this.getMetaEvents();
		} catch {
			this.isRetryingMetaEvent = false;
		}
	}

	routeToMetaEvents() {
		this.router.navigateByUrl('/projects/' + this.privateService.getProjectDetails?.name + '/settings?activePage=meta%20events');
	}
}
