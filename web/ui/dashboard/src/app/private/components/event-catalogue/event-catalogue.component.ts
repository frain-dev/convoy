import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { GeneralService } from 'src/app/services/general/general.service';
import { ActivatedRoute } from '@angular/router';
import { EventsCatalogueService } from 'src/app/public/events-catalogue/events-catalogue.service';
import { CreateProjectComponentService } from '../create-project-component/create-project-component.service';
import { SkeletonLoaderComponent } from 'src/app/components/skeleton-loader/skeleton-loader.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { CatalogueComponent } from 'src/app/components/catalogue/catalogue.component';
import { PrismModule } from '../prism/prism.module';

@Component({
    selector: 'convoy-event-catalogue',
    standalone: true,
    imports: [CommonModule, SkeletonLoaderComponent, TagComponent, CatalogueComponent, PrismModule],
    templateUrl: './event-catalogue.component.html',
    styleUrls: ['./event-catalogue.component.scss']
})
export class EventCatalogueComponent implements OnInit {
    eventsCatalogue: any;
    fetchingCatalogue = true;
    selectedProperty!: string;
    token = this.route.snapshot.queryParams.token;


    constructor(public generalService: GeneralService, private route: ActivatedRoute, private eventCatalogueService: EventsCatalogueService, private createProjectService: CreateProjectComponentService) { }

    ngOnInit() {
        this.getCatalogue();
    }

    async getCatalogue() {
        try {
            const response = this.token ? await this.eventCatalogueService.getEventCatlogue() : await this.createProjectService.getEventCatalogue();
            const { data } = response;
            if (data.type === 'events_data') await this.processEventsData(data.events);
        } catch { }
    }

    async processEventsData(eventsData: any) {
        try {
            const processedEvents = await this.eventCatalogueService.processJSONEvent(eventsData);
            this.eventsCatalogue = processedEvents;
            this.fetchingCatalogue = false;
        } catch { }
    }

}
