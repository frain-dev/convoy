import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { PrismModule } from 'src/app/private/components/prism/prism.module';
import { GeneralService } from 'src/app/services/general/general.service';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { EventsCatalogueService } from './events-catalogue.service';
import { CatalogueComponent } from 'src/app/components/catalogue/catalogue.component';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';

@Component({
	selector: 'convoy-events-catalogue',
	standalone: true,
	imports: [CommonModule, PrismModule, TagComponent, CatalogueComponent, SkeletonLoaderComponent],
	templateUrl: './events-catalogue.component.html',
	styleUrls: ['./events-catalogue.component.scss']
})
export class EventsCatalogueComponent implements OnInit {
	eventsCatalogue: any;
	fetchingCatalogue = true;
	selectedProperty!: string;

	constructor(public generalService: GeneralService, private eventCatalogueService: EventsCatalogueService) {}

	ngOnInit() {
		this.getCatalogue();
	}

	async getCatalogue() {
		try {
			const response = await this.eventCatalogueService.getEventCatlogue();
			const { data } = response;
			if (data.type === 'events_data') await this.processEventsData(data.events);
		} catch {}
	}

	async processEventsData(eventsData: any) {
		try {
			const processedEvents = await this.eventCatalogueService.processJSONEvent(eventsData);
			this.eventsCatalogue = processedEvents;
			this.fetchingCatalogue = false;
			console.log(this.eventsCatalogue);
		} catch {}
	}
}
