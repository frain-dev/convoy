import { Component, EventEmitter, Input, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ModalComponent } from 'src/app/components/modal/modal.component';
import { CopyButtonComponent } from 'src/app/components/copy-button/copy-button.component';
import { ButtonComponent } from 'src/app/components/button/button.component';

@Component({
	selector: 'convoy-token-modal',
	standalone: true,
	imports: [CommonModule, ModalComponent, CopyButtonComponent, ButtonComponent],
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
