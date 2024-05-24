import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ActivatedRoute, Router, RouterModule } from '@angular/router';
import { EventDeliveryDetailsModule } from 'src/app/private/pages/project/events/event-delivery-details/event-delivery-details.module';
import { ButtonComponent } from 'src/app/components/button/button.component';

@Component({
	selector: 'convoy-event-delivery',
	standalone: true,
	imports: [CommonModule, EventDeliveryDetailsModule, RouterModule, ButtonComponent],
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
