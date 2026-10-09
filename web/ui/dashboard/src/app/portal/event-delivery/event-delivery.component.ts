import { Component, OnInit } from '@angular/core';

import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { EventDeliveryDetailsModule } from 'src/app/private/pages/project/events/event-delivery-details/event-delivery-details.module';

@Component({
    selector: 'convoy-event-delivery',
    imports: [EventDeliveryDetailsModule, RouterModule],
    templateUrl: './event-delivery.component.html',
    styleUrls: ['./event-delivery.component.scss']
})
export class EventDeliveryComponent implements OnInit {
	portalToken = this.route.snapshot.queryParams.token;

	constructor(private route: ActivatedRoute, private router: Router) {}

	ngOnInit(): void {}

	viewEndpointDetails(endpointId: string) {
		this.router.navigate(['/portal'], { queryParams: { token: this.portalToken, endpointId: endpointId } });
	}
}
