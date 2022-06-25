import { Component, OnInit } from '@angular/core';

@Component({
	selector: 'app-create-subscription-public',
	templateUrl: './create-subscription.component.html',
	styleUrls: ['./create-subscription.component.scss']
})
export class CreateSubscriptionComponent implements OnInit {
	constructor() {}

	ngOnInit(): void {}

	closeCreateSubscriptionModal() {
		window.parent.document.querySelector('#convoy-create-subscription-modal')!.remove();
	}
}
