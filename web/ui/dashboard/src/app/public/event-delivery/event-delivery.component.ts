import { Component, OnInit } from '@angular/core';
import { ActivatedRoute, Router } from '@angular/router';

@Component({
	selector: 'app-event-delivery',
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
