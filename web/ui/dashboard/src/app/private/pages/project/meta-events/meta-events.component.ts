import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrivateService } from 'src/app/private/private.service';
import { CardComponent } from 'src/app/components/card/card.component';
import { EmptyStateComponent } from 'src/app/components/empty-state/empty-state.component';
import { ButtonComponent } from 'src/app/components/button/button.component';
import { TooltipComponent } from 'src/app/components/tooltip/tooltip.component';
import { ModalComponent, ModalHeaderComponent } from 'src/app/components/modal/modal.component';
import { MetaEventsService } from './meta-events.service';
import { TableLoaderModule } from 'src/app/private/components/table-loader/table-loader.module';
import { TableCellComponent, TableComponent, TableHeadCellComponent, TableHeadComponent, TableRowComponent } from 'src/app/components/table/table.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { PrismModule } from 'src/app/private/components/prism/prism.module';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';

@Component({
	selector: 'convoy-meta-events',
	standalone: true,
	imports: [
		CommonModule,
		CardComponent,
		EmptyStateComponent,
		ButtonComponent,
		TooltipComponent,
		ModalComponent,
		ModalHeaderComponent,
		TableLoaderModule,
		TableCellComponent,
		TableComponent,
		TableHeadComponent,
		TableHeadCellComponent,
		TableRowComponent,
		TagComponent,
		StatusColorModule,
		PrismModule
	],
	templateUrl: './meta-events.component.html',
	styleUrls: ['./meta-events.component.scss']
})
export class MetaEventsComponent implements OnInit {
	metaEventsTableHead: string[] = ['Status', 'Event Types', 'Retries', 'Time', '', ''];
	showMetaConfig = false;
	isLoadingMetaEvents = false;
	isRetryingMetaEvent = false;
	displayedMetaEvents: any;
	selectedMetaEvent: any;

	constructor(public privateService: PrivateService, private generalService: GeneralService, private metaEventsService: MetaEventsService) {}

	ngOnInit(): void {
		this.getMetaEvents();
	}

	get isMetaEventEnabled(): Boolean {
		const isMetaEventEnabled = localStorage.getItem('IS_META_EVENTS_ENABLED');
		if (isMetaEventEnabled) return JSON.parse(isMetaEventEnabled);
		return false;
	}

	toggleMetaConfig(event: any) {
		const isConfigfureMetaEventsChecked = event.target.checked;
		this.showMetaConfig = isConfigfureMetaEventsChecked;
	}

	async getMetaEvents() {
		this.isLoadingMetaEvents = true;
		try {
			const response = await this.metaEventsService.getMetaEvents();
			const metaEvents = response.data.content;
			if (metaEvents.length) this.selectedMetaEvent = metaEvents[0];
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

	getCodeSnippetString(type: 'req_header' | 'res_header' | 'res_body') {
		if (type === 'req_header') {
			if (!this.selectedMetaEvent?.attempt?.request_http_header) return 'No request data was sent';
			return JSON.stringify(this.selectedMetaEvent?.attempt?.request_http_header, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		}
		if (type === 'res_header') {
			if (!this.selectedMetaEvent?.attempt?.response_http_header) return 'No response header was sent';
			return JSON.stringify(this.selectedMetaEvent?.attempt?.response_http_header, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		}
		if (type === 'res_body') {
			if (!this.selectedMetaEvent?.metadata?.data) return 'No response header was sent';
			return JSON.stringify(this.selectedMetaEvent?.metadata?.data, null, 4).replaceAll(/"([^"]+)":/g, '$1:');
		}
		return '';
	}
}
