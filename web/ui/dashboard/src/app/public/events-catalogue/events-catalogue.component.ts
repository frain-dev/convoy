import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { EventCatalogueComponent } from 'src/app/private/components/event-catalogue/event-catalogue.component';

@Component({
    selector: 'convoy-events-catalogue',
    standalone: true,
    imports: [CommonModule, EventCatalogueComponent],
    templateUrl: './events-catalogue.component.html',
    styleUrls: ['./events-catalogue.component.scss']
})
export class EventsCatalogueComponent implements OnInit {

    constructor() { }

    ngOnInit() { }

}
