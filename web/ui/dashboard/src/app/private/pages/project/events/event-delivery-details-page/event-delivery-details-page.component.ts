import { Component, OnInit } from '@angular/core';
import { PrivateService } from 'src/app/private/private.service';
import { Location } from '@angular/common';

@Component({
	selector: 'app-event-delivery-details-page',
	templateUrl: './event-delivery-details-page.component.html',
	styleUrls: ['./event-delivery-details-page.component.scss']
})
export class EventDeliveryDetailsPageComponent implements OnInit {
	constructor(public privateService: PrivateService, private location: Location) {}

	ngOnInit(): void {}

	goBack() {
		this.location.back();
	}
}
