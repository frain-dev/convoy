import { Component, ElementRef, OnInit, ViewChild } from '@angular/core';

@Component({
	selector: 'app-create-subscription-public',
	templateUrl: './create-subscription.component.html',
	styleUrls: ['./create-subscription.component.scss']
})
export class CreateSubscriptionComponent implements OnInit {
	@ViewChild('dialog', { static: true }) dialog!: ElementRef<HTMLDialogElement>;

	constructor() {
		this.dialog.nativeElement.showModal();
	}

	ngOnInit(): void {}

	closeCreateSubscriptionModal() {
		this.dialog.nativeElement.close();
		window.parent.document.querySelector('#convoy-create-subscription-modal')!.remove();
	}
}
