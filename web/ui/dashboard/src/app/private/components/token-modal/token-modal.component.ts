import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';

import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';

@Component({
    selector: 'convoy-token-modal',
    imports: [CopyButtonComponent],
    templateUrl: './token-modal.component.html',
    styleUrls: ['./token-modal.component.scss']
})
export class TokenModalComponent implements OnInit {
	@Input('title') title!: string;
	@Input('description') description!: string;
	@Input('token') token!: string;
	@Input('notificationText') notificationText!: string;
	@Output() closeModal = new EventEmitter<any>();

	constructor() {}

	ngOnInit(): void {}
}
